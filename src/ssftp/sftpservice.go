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
			//https://blog.gopheracademy.com/advent-2015/ssh-server-in-go/
			//https://security.stackexchange.com/questions/9366/ssh-public-private-key-pair/9389#9389
			//https://github.com/bored-engineer/ssh/commit/1b71c35864fb15ae4623d2f63ddb0a508e7038ec
			
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

	ss.acceptConns(config)
}

func (ss *SFTPService) acceptConns(svrConfig *ssh.ServerConfig) {
	for {
		
		newConn, err := ss.netListener.Accept()

		if err != nil {
			logclient.ErrIfm("Failed to accept incoming SSH connection", err)

			continue
		}

		go ss.handleConnectingClients(newConn, svrConfig)
	}
}

func (ss *SFTPService) handleConnectingClients(conn net.Conn, svrConfig *ssh.ServerConfig) {

	debugStream := os.Stderr

	go func() {
		time.Sleep(600 * time.Second)

		logclient.Info("SFTPService - Client handshake took more than 10mins, timing out")

		conn.Close()

	}()

	// Before use, a handshake must be performed on the incoming
	// net.Conn.
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
			logclient.Info("Channel accepted\n")
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

	//remove pem file
	// os.Remove("private.pem")
	// if err != nil {
	// 	logclient.ErrIfm("Failed to delete private.pem", ppkerr)
	// }

	return sshSigner
}

// func (ss SFTPService) createUserDir(userName string) {

// 	dirPath := filepath.Join(ss.configsvc.config.StagingPath, userName)

// 	if !isDirExist(dirPath) {
// 		err := os.Mkdir(dirPath, os.FileMode(0775))
// 		if err != nil {
// 			logclient.ErrIfm(fmt.Sprintf("Error while creating directory for user %s", userName), err)
// 			return
// 		}

// 		ss.chownDir(dirPath, userName)
// 	} else {
// 		logclient.Infof("Directory exist for %s", dirPath)
// 	}
// }

// func (ss SFTPService) chownDir(dir string, userName string) {

// 	logclient.Infof("Executing Chown for dir:%s with user:%s", dir, userName)

// 	if runtime.GOOS != "windows" {
// 		group, err := user.Lookup(userName)
// 		if err != nil {
// 			logclient.ErrIfm(("error looking up user"), err)
// 			return
// 		}

// 		uid, _ := strconv.Atoi(group.Uid)
// 		gid, _ := strconv.Atoi(group.Gid)
	
// 		cmerr := os.Chmod(dir, os.FileMode(0777))
// 		logclient.ErrIfm(fmt.Sprintf("Error changing dir mode"), cmerr)

// 		cerr := os.Chown(dir, uid, gid)
// 		logclient.ErrIfm(fmt.Sprintf("Error changing dir owner"), cerr)
		
	
// 		if cerr != nil {
// 			logclient.ErrIfm(fmt.Sprintf("error while Chown on dir: %s with user: %s", dir,  userName), cerr)
// 			return
// 		}
// 		logclient.Infof("Executing Chown for dir:%s with user:%s completed successfully", dir, userName)

		
// 	}
// }








// type Route struct {
// 	Username string `json:"username"`
// 	Password string `json:"password"`
// 	Endpoint string `json:"endpoint"`
// 	Local    bool   `json:"local"`
// }

// type WriteNotification struct {
// 	Username string
// 	Path     string
// }


// type SftpService struct {
// 	usergov  		   UserGov
// 	loginUser			User
// 	routes             []Route
// 	routesMutex        *sync.RWMutex
// 	privateKey         ssh.Signer
// 	host               string
// 	port               int
// 	chroot             string
// 	incoming           chan sftp.WrittenFile
// 	writeNotifications chan WriteNotification
// 	listener           net.Listener
// 	servers            map[string]*sftp.Server
// 	serversMutex       *sync.RWMutex
// 	quit               chan bool
// }

// func NewSftpService(host string, port int, chroot string, routes []Route, privateKey ssh.Signer, usrgov UserGov) *SftpService {
// 	return &SftpService{
// 		usergov:			usrgov,
// 		loginUser:			User{},
// 		routes:             routes,
// 		routesMutex:        &sync.RWMutex{},
// 		privateKey:         privateKey,
// 		host:               host,
// 		port:               port,
// 		chroot:             chroot,
// 		incoming:           make(chan sftp.WrittenFile, 100),
// 		writeNotifications: make(chan WriteNotification, 100),
// 		servers:            make(map[string]*sftp.Server),
// 		serversMutex:       &sync.RWMutex{},
// 		quit:               make(chan bool, 1),
// 	}
// }

// func (s *SftpService) Start() error {
	

// 		config := &ssh.ServerConfig{
// 		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
// 			// Should use constant-time compare (or better, salt+hash) in
// 			// a production setting.

// 			return nil, nil
			
// 			if usr, ok := s.usergov.Auth(c.User(), string(pass)); ok {
				
// 				logclient.Infof("User %s has signed in", usr.Name)

// 				// s.loginUser = usr

// 				// s.usergov.CreateUserDir(usr.Directory)
				

// 				return nil, nil
// 			}
// 			return nil, fmt.Errorf("password rejected for %q", c.User())
// 		},
// 	}

// 	config.AddHostKey(s.genSSHKey())

// 	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
// 	if err != nil {
// 		return err
// 	}

// 	s.listener = listener

// 	go s.accept(config)
// 	go s.watchIncoming()

// 	logclient.Infof("SFTP server started, listening to port %s", strconv.Itoa(s.port))

// 	return nil
// }

// func (s *SftpService) accept(config *ssh.ServerConfig) {
// 	for {
// 		logclient.Info("New connection")

// 		newConn, err := s.listener.Accept()
// 		if err != nil {
// 			select {
// 			case <-s.quit:
// 				return
// 			default:
// 			}

// 			logclient.ErrIfm("Failed to accept incoming SSH connection", err)
// 			continue
// 		}

// 		go s.handleClient(newConn, config)
// 	}
// }

// func (s *SftpService) handleClient(conn net.Conn, config *ssh.ServerConfig) {
// 	sessionOpen := false

// 	go func() {
// 		time.Sleep(8 * time.Second)

// 		if !sessionOpen {
// 			logclient.Infof("Client %s handshake took too long, timing out", conn.RemoteAddr())
// 			conn.Close()
// 		}
// 	}()

// 	//Before use, a handshake must be performed on the incoming net.Conn.
// 	serverConn, chans, reqs, err := ssh.NewServerConn(conn, config)
// 	if err != nil {
// 		if err != io.EOF {
// 			logclient.ErrIfm("Failed to handshake SSH connection", err)
// 		}

// 		return
// 	}

// 	defer serverConn.Close()

// 	logclient.Info("Handshake complete")

// 	//The incoming Request channel must be serviced.
// 	go ssh.DiscardRequests(reqs)

// 	serverID := string(serverConn.SessionID())

// 	//Service the incoming Channel channel.
// 	for newChannel := range chans {
// 		// Channels have a type, depending on the application level
// 		// protocol intended. In the case of an SFTP session, this is "subsystem"
// 		// with a payload string of "<length=4>sftp"
// 		if newChannel.ChannelType() != "session" {
// 			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
// 			continue
// 		}

// 		channel, requests, err := newChannel.Accept()
// 		if err != nil {
// 			logclient.ErrIfm("Could not accept channel", err)
// 			break
// 		}

// 		sessionOpen = true

// 		//Sessions have out-of-band requests such as "shell",
// 		// "pty-req" and "env".  Here we handle only the
// 		// "subsystem" request.
// 		go func(in <-chan *ssh.Request) {
// 			for req := range in {
// 				ok := false
// 				switch req.Type {
// 				case "subsystem":
// 					if string(req.Payload[4:]) == "sftp" {
// 						ok = true
// 					}
// 				}

// 				req.Reply(ok, nil)
// 			}
// 		}(requests)

// 		//!important, sftp uses "AsUser" to chroot user to folder.
// 		//Default folder name = username. Changing to support multi-user same folder
// 		serverOptions := []sftp.ServerOption{

// 			sftp.Chroot(s.chroot),
// 			sftp.NotifyWrite(s.incoming),
// 			sftp.AsUser(serverConn.User()), //(s.loginUser.Directory), //
// 		}

// 		server, err := sftp.NewServer(channel, serverOptions...)
// 		if err != nil {
// 			logclient.ErrIfm("Failed to create new SFTP server instance", err)
// 			break
// 		}

// 		s.serversMutex.Lock()
// 		s.servers[serverID] = server
// 		s.serversMutex.Unlock()

// 		if err := server.Serve(); err != nil {
// 			if err != io.EOF {
// 				logclient.ErrIfm("SFTP server instance crashed", err)
// 			}

// 			break
// 		}
// 	}

// 	s.serversMutex.Lock()
// 	delete(s.servers, serverID)
// 	s.serversMutex.Unlock()

// 	logclient.Info("Connection closed")
// }

// func (s *SftpService) watchIncoming() {
// 	for writtenFile := range s.incoming {
// 		notification := WriteNotification{
// 			Username: writtenFile.User,
// 			Path:     writtenFile.Path,
// 		}

// 		logclient.Infof("User %s uploads file %s", writtenFile.User, writtenFile.Path)
		
// 		s.writeNotifications <- notification
// 	}
// }

// func (s *SftpService) WriteNotifications() chan WriteNotification {
// 	return s.writeNotifications
// }

