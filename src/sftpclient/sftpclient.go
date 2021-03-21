package sftpclient

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPClient struct {
	Host string
	Port int
	DeleteRemoteFileAfterDownload bool
	OverrideExistingFile bool
	Username string
	Password string
	PrivateKeyPath string
	LocalStagingDirectory string
	RemoteDirectory string
	sftpClient *sftp.Client
}

func (sftpc *SFTPClient) NewClient(host string, port int, username string, pass string, privatekeyPath string, localStagingDir string, remoteDir string, deleteRemoteFileAfterDownload bool, overrideExistingFile bool) *SFTPClient {
	return &SFTPClient{
		Host: host,
		Port: port,
		DeleteRemoteFileAfterDownload: deleteRemoteFileAfterDownload,
		OverrideExistingFile: overrideExistingFile,
		Username: username,
		Password: pass,
		PrivateKeyPath: privatekeyPath,
		LocalStagingDirectory: localStagingDir,
		RemoteDirectory: remoteDir,
	}
}

func (sftpc *SFTPClient) DownloadFilesRecursive() (error) {
	walker :=  sftpc.sftpClient.Walk(sftpc.RemoteDirectory)

	for walker.Step() {
		if walker.Err() != nil {
			continue
		}

		if walker.Stat().IsDir() { //sync remote and local dir structure
			sftpc.createLocalDir(walker.Path())
			continue
		}

		rmtFilePath := walker.Path()
		 
		localFilePath := filepath.Join(sftpc.LocalStagingDirectory, filepath.Base(rmtFilePath))

		err := sftpc.createLocalFile(localFilePath)
		if err != nil {
			return err
		}

		sftpc.copyBytesFromRemoteToLocalFile(rmtFilePath, localFilePath)

		// files, err := sftpc.sftpClient.ReadDir(walker.Path())
		// if err != nil {
		// 	return err
		// }
		
		// for _, f := range files {
		// 	rmtFile, err := sftpc.sftpClient.Open(f.Name())
		// 	if err != nil {
		// 		return err
		// 	}

			

		// 	if !sftpc.isDirFileExist(rmtFile.Name()) {

		// 		if sftpc.OverrideExistingFile {

		// 		}

		// 	} else {
				
		// 	}
		// }
	}

	return nil
}

func (sftpc *SFTPClient) Connect() (error) {

	pkAuthMethod, err := sftpc.newPublicKeyAuthMethod()
	if err != nil {
		return err
	}
	
	authMs := make([]ssh.AuthMethod, 0)
	authMs = append(authMs, pkAuthMethod)
	authMs = append(authMs, ssh.Password(sftpc.Password))

	config := &ssh.ClientConfig{
		User:            sftpc.Username,
		Auth:           authMs,
		Timeout:         10 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	config.Ciphers = append(config.Config.Ciphers, "aes128-gcm@openssh.com")

	addr := fmt.Sprintf("%s:%d", sftpc.Host, sftpc.Port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}

	sftpc.sftpClient = client

	return nil
}

func (sftpc *SFTPClient) createLocalDir(dir string) {
	if sftpc.isDirFileExist(dir) {
		os.Mkdir(dir, 0755)
	}
}

func (sftpc *SFTPClient) createLocalFile(file string) (error) {
	if sftpc.isDirFileExist(file) {
		_, err := os.Create(file)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sftpc *SFTPClient) isDirFileExist(path string) (bool) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func (sftpc *SFTPClient) copyBytesFromRemoteToLocalFile(destFile string, localFile string) (int64, error) {
	destF, err := os.Open(destFile)
	if err != nil {
		return 0, err
	}

	localF, err := os.Open(localFile)
	if err != nil {
		return 0, err
	}

	bc, cerr := io.Copy(destF, localF)
	if cerr != nil {
		return bc, cerr
	}

	return bc, nil
}

func (sftpc *SFTPClient) newPublicKeyAuthMethod() (ssh.AuthMethod, error) {
	pemBytes, err := ioutil.ReadFile(sftpc.PrivateKeyPath)
    if err != nil {
        return nil, err
    }
    signer, err := ssh.ParsePrivateKey(pemBytes)
    if err != nil {
		return nil, err
    }

	return ssh.PublicKeys(signer), nil
}