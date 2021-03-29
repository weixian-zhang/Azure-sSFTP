package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"
	"strconv"
	sftp "github.com/weixian-zhang/ssftp/pkgsftp"
	"github.com/weixian-zhang/ssftp/user"
	"golang.org/x/crypto/ssh"
)

type SFTPService struct {
	configsvc *ConfigService
	usrgov 		*user.UserGov
	loginUser   user.User
	netListener  net.Listener
	servers		[]*sftp.Server
}

func NewSFTPService(configsvc *ConfigService, usrgov *user.UserGov) (SFTPService) {
	return SFTPService{
		configsvc: configsvc,
		usrgov: usrgov,
		loginUser: user.User{},
		servers: make([]*sftp.Server, 0),
	}
}

func (ss *SFTPService) Start() {

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			
			logclient.Infof("User %s attempting password authentication", conn.User())

			if usr, ok := ss.usrgov.AuthPass(conn.User(), string(pass)); ok {

				ss.loginUser = usr
				
				if ss.loginUser.IsCleanDirUser {
					if ss.loginUser.JailDirectory != "*" {
						ss.usrgov.CreateUserDir(ss.configsvc.config.CleanPath, ss.loginUser.JailDirectory)
					}
				} else {
					ss.usrgov.CreateUserDir(ss.configsvc.config.StagingPath, ss.loginUser.JailDirectory)
				}
			
				return nil, nil
			}

			return nil, fmt.Errorf("password rejected for %q", conn.User())
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			
			logclient.Infof("User %s attempting certificate authentication", conn.User())

			usr, ok, err :=  ss.usrgov.AuthPublicKey(conn.User(), pubKey)

			if logclient.ErrIf(err) {
				return nil, err
			}

			if ok {
				
				logclient.Infof("User %s has logged in successfully using certificate authentication", conn.User())

				ss.loginUser = usr

				if ss.loginUser.IsCleanDirUser {
					if ss.loginUser.JailDirectory != "*" {
						ss.usrgov.CreateUserDir(ss.configsvc.config.CleanPath, ss.loginUser.JailDirectory)
					}
				} else {
					ss.usrgov.CreateUserDir(ss.configsvc.config.StagingPath, ss.loginUser.JailDirectory)
				}

				return nil, nil
			} else {
				return nil, fmt.Errorf("Public Key authentication is unsuccessful for %s", conn.User())
			}
		},
	}


	config.AddHostKey(ss.genSSHKey())

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", strconv.Itoa(ss.configsvc.config.SftpPort)))
	if err != nil {
		logclient.ErrIfm("failed to listen for connection", err)
	}
	logclient.Infof("SFTP server listening on %v", listener.Addr())

	ss.netListener = listener

	go ss.acceptConns(config)
}

func (ss *SFTPService) acceptConns(svrConfig *ssh.ServerConfig) {

	for {
		
		logclient.Infof("SftpService - awaiting client connection")

		newConn, err := ss.netListener.Accept()
		if err != nil {
			logclient.ErrIfm("SftpService - error whike Listener accepting connection", err)
		}
		
		logclient.Infof("SftpService - accepted Sftp client from %s, proceeding to authentication", newConn.RemoteAddr())

		go ss.handleConnectingClients(newConn, svrConfig)
	}
}

func (ss *SFTPService) handleConnectingClients(conn net.Conn, svrConfig *ssh.ServerConfig) {
	//defer conn.Close()

	debugStream := os.Stderr

	rerr := conn.SetReadDeadline(time.Now().Add(time.Duration(UploadTimeLimitMin) * time.Minute))
	logclient.ErrIfm("SFTPService - Error while setting read deadline", rerr)

	werr := conn.SetWriteDeadline(time.Now().Add(time.Duration(UploadTimeLimitMin) * time.Minute))
	logclient.ErrIfm("SFTPService - Error while setting write deadline", werr)

	// Before use, a handshake must be performed on the incoming
	_, chans, reqs, err := ssh.NewServerConn(conn, svrConfig)
	if err != nil {
		logclient.ErrIfm("SFTPService - failed to handshake", err)
	}
	fmt.Fprintf(debugStream, "SFTPService - SSH server established\n")

	// The incoming Request channel must be serviced.
	go ssh.DiscardRequests(reqs)

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of an SFTP session, this is "subsystem"
		// with a payload string of "<length=4>sftp"
		//fmt.Fprintf(debugStream, "Incoming channel: %s\n", newChannel.ChannelType())

		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			fmt.Fprintf(debugStream, "SFTPService - Unknown channel type: %s\n", newChannel.ChannelType())
			continue
		}

		channel, requests, err := newChannel.Accept()
		
		if err != nil {
			logclient.ErrIfm("SFTPService - could not accept channel.", err)
		} else {
			logclient.Info("SftpService - Channel accepted")
		}
		

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				
				fmt.Fprintf(debugStream, "SFTPService - Request: %v\n", req.Type)
				ok := false
				switch req.Type {
				case "subsystem":
					fmt.Fprintf(debugStream, "SFTPService - Subsystem: %s\n", req.Payload[4:])
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}
				fmt.Fprintf(debugStream, " - accepted: %v\n", ok)
				req.Reply(ok, nil)
			}
		}(requests)

		
		serverOptions := []sftp.ServerOption{
			sftp.WithDebug(debugStream),
		}
		
		//TODO: readonly option
		// if readOnly {
		// 	serverOptions = append(serverOptions, sftp.ReadOnly())
		// 	fmt.Fprintf(debugStream, "Read-only server\n")
		// } else {
		// 	fmt.Fprintf(debugStream, "Read write server\n")
		// }

		var jailPath string
		
		if ss.loginUser.IsCleanDirUser {
			jailPath = ss.configsvc.config.CleanPath
		} else {
			jailPath = ss.configsvc.config.StagingPath
		}

		server, err := sftp.NewServer(
			channel,
			ss.loginUser,
			jailPath,
			serverOptions...,
		)

		if err != nil {
			logclient.ErrIf(err)
		}
		
		ss.servers = append(ss.servers, server)

		if err := server.Serve(); err == io.EOF {
		    server.Close()
			logclient.Infof("sftp client %s exited session", server.User.Name)

			logclient.Infof("Removed sftp.Server connection %s", server.User.Name)
			ss.removeServer(server)

			if len(ss.servers) == 0 {
				logclient.Info("No active sftp client")
			}

		} else if err != nil {
			logclient.ErrIfm("sftp server completed with error:", err)
		}
	}

}

func (ss *SFTPService) removeServer(server *sftp.Server) {
	for i := len(ss.servers) -1; i > 0; i-- {
		if ss.servers[i] == server {
			ss.servers = append(ss.servers[:i], ss.servers[i+1:]...)
		}
	}
}

func (ss *SFTPService) genSSHKey() (ssh.Signer) {

	if _ , err := os.Stat("private.pem"); os.IsExist(err) {
		
	}
	
	keyPath := "/mnt/ssftp/system/private.pem"

	if isWindows() {
		keyPath = "private.pem"
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {

		key, err :=  rsa.GenerateKey(rand.Reader, 2048)
		if logclient.ErrIf(err) {
			logclient.ErrIf(errors.New("sSFTP fail to generate RSA key"))
			return nil
		}
		
		pemBytes := x509.MarshalPKCS1PrivateKey(key)
		privateKeyBlock := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: pemBytes,
		}

		privatePem, err := os.Create(keyPath)
		if err != nil {
			logclient.ErrIf(err)
			os.Exit(1)
		}
		err = pem.Encode(privatePem, privateKeyBlock)
		if err != nil {
			logclient.ErrIfm("error when encode private pem: %s \n", err)
			os.Exit(1)
		} 
	}
	
	pemPKeyFile, err := ioutil.ReadFile(keyPath)
	if err != nil {
		logclient.ErrIfm("Failed to load private key", err)
	}

	sshSigner, ppkerr := ssh.ParsePrivateKey(pemPKeyFile)
	if err != nil {
		logclient.ErrIfm("Failed to parse as ssh key", ppkerr)
		return nil
	}

	return sshSigner
}


