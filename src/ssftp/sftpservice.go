 package main

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"
	"net"
	"os"
	//"os/user"
	//"runtime"
	sftp "github.com/weixian-zhang/ssftp/pkgsftp"
	//"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"crypto/x509"
	"encoding/pem"
	"github.com/weixian-zhang/ssftp/user"
)

// https://github.com/pkg/sftp/blob/master/examples/go-sftp-server/main.go

// https://github.com/atmoz/sftp/blob/master/files/create-sftp-user


// shuttle SFTP example: https://github.com/TaitoUnited/shuttle/blob/master/sftpservice.go
// new SFTP antipaste: https://github.com/AntiPaste/sftp/blob/master/server.go

type SFTPService struct {
	configsvc *ConfigService
	usrgov 		user.UserGov
	netListener  net.Listener
}

func NewSFTPService(configsvc *ConfigService, usrgov user.UserGov) (SFTPService) {
	return SFTPService{
		configsvc: configsvc,
		usrgov: usrgov,
	}
}

func (ss SFTPService) Start() {

	debugStream := os.Stderr
	
// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in
			// a production setting.
			fmt.Fprintf(debugStream, "Login: %s\n", c.User())
			if c.User() == "testuser" && string(pass) == "tiger" {
				
				// ss.usergov.createAdminGroup()
				// ss.usergov.AddNewUser(filepath.Join(ss.config.StagingPath,"testuser"), "testuser", "tiger")
				// ss.createUserDir("testuser")

				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}


	config.AddHostKey(ss.genSSHKey())

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:22")
	if err != nil {
		logclient.ErrIfm("failed to listen for connection", err)
	}
	logclient.Infof("SFTP server listening on %v\n", listener.Addr())

	ss.netListener = listener

	ss.acceptConns(config)

	// newConn, err := listener.Accept() //waits until a connection is accepted
	// if err != nil {
	// 	logclient.ErrIfm("failed to accept incoming connection", err)
	// }

	

}

func (ss SFTPService) acceptConns(svrConfig *ssh.ServerConfig) {
	for {
		
		newConn, err := ss.netListener.Accept()

		if err != nil {
			logclient.ErrIfm("Failed to accept incoming SSH connection", err)

			continue
		}

		go ss.handleConnectingClients(newConn, svrConfig)
	}
}

func (ss SFTPService) handleConnectingClients(conn net.Conn, svrConfig *ssh.ServerConfig) {

	debugStream := os.Stderr

	go func() {
		time.Sleep(20 * time.Second)

		logclient.Info("Handshake took too long, timing out")

		conn.Close()

	}()

	// Before use, a handshake must be performed on the incoming
	// net.Conn.
	_, chans, reqs, err := ssh.NewServerConn(conn, svrConfig)
	if err != nil {
		logclient.ErrIfm("failed to handshake", err)
	}
	fmt.Fprintf(debugStream, "SSH server established\n")

	// The incoming Request channel must be serviced.
	go ssh.DiscardRequests(reqs)

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of an SFTP session, this is "subsystem"
		// with a payload string of "<length=4>sftp"
		fmt.Fprintf(debugStream, "Incoming channel: %s\n", newChannel.ChannelType())
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			fmt.Fprintf(debugStream, "Unknown channel type: %s\n", newChannel.ChannelType())
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			logclient.ErrIfm("could not accept channel.", err)
		} else {
			logclient.Info("Channel accepted\n")
		}
		

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				
				fmt.Fprintf(debugStream, "Request: %v\n", req.Type)
				ok := false
				switch req.Type {
				case "subsystem":
					fmt.Fprintf(debugStream, "Subsystem: %s\n", req.Payload[4:])
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}
				fmt.Fprintf(debugStream, " - accepted: %v\n", ok)
				req.Reply(ok, nil)
			}
		}(requests)

		//userRootDir := filepath.Join(ss.configsvc.config.StagingPath,"testuser")
		
		serverOptions := []sftp.ServerOption{
			sftp.WithDebug(debugStream),
		}
		
		// if readOnly {
		// 	serverOptions = append(serverOptions, sftp.ReadOnly())
		// 	fmt.Fprintf(debugStream, "Read-only server\n")
		// } else {
		// 	fmt.Fprintf(debugStream, "Read write server\n")
		// }

		server, err := sftp.NewServer(
			channel,
			serverOptions...,
		)
		if err != nil {
			logclient.ErrIf(err)
		}
		if err := server.Serve(); err == io.EOF {
			server.Close()
			logclient.Info("sftp client exited session.")
		} else if err != nil {
			logclient.ErrIfm("sftp server completed with error:", err)
		}
	}

}

func (ss SFTPService) genSSHKey() (ssh.Signer) {

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


