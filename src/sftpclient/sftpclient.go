package sftpclient

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
	"github.com/pkg/sftp"
	"github.com/weixian-zhang/ssftp/logc"
	"golang.org/x/crypto/ssh"
)

type SFTPClient struct {
	DownloaderConfig *DownloaderConfig
	sftpClient *sftp.Client
	logclient *logc.LogClient
}

type DownloaderConfig struct {
	Host string
	Port int
	DeleteRemoteFileAfterDownload bool
	OverrideExistingFile bool
	Username string
	Password string
	PrivateKeyPath string
	LocalStagingBaseDirectory string
	LocalStagingDirectory string
	RemoteDirectory string
	IsConnectedToServer bool
	fullLocalStagingDir string
	fullRemoteDir string
}

func NewSftpClient(config *DownloaderConfig, logclient *logc.LogClient) *SFTPClient {
	return &SFTPClient{
		DownloaderConfig: config,
		sftpClient: nil,
		logclient: logclient,
	}
}


func (sftpc *SFTPClient) DownloadFilesRecursive() (error) {

	sftpc.setFullLocalStagingAndRemotePath()

	sftpc.createLocalDir(sftpc.DownloaderConfig.fullLocalStagingDir)
	
	walker :=  sftpc.sftpClient.Walk(sftpc.DownloaderConfig.fullRemoteDir)

	for walker.Step() {
		if walker.Err() != nil {
			sftpc.logclient.ErrIfm("Sftpclient - error while directory walking", walker.Err())
			continue
		}

		if walker.Stat().IsDir() { //sync remote and local dir structure
			sftpc.createLocalDir(walker.Path())
			continue
		}

		rmtFilePath := walker.Path()
		 
		localFilePath := filepath.Join(sftpc.DownloaderConfig.LocalStagingDirectory, filepath.Base(rmtFilePath))

		err := sftpc.createLocalFile(localFilePath)
		if err != nil {
			return err
		}

		byteCopied, err := sftpc.copyBytesFromRemoteToLocalFile(rmtFilePath, localFilePath)

		if err != nil {
			sftpc.logclient.ErrIffmsg("SFTPClient - error while downloading file from %s", err, rmtFilePath)
			return err
		}

		sftpc.logclient.Infof("SFTPClient - file %s downloaded successfully, size: %d", rmtFilePath, byteCopied)
	
	}

	return nil
}

func (sftpc *SFTPClient) Connect() (error) {

	authMs := make([]ssh.AuthMethod, 0)

	pkAuthMethod, err := sftpc.newPublicKeyAuthMethod()
	if err != nil {
		sftpc.logclient.ErrIfm("SFTPClient - error occur while reading private key file. Ignoring Private Key authn.", err)
		//return err
	} else {
		authMs = append(authMs, pkAuthMethod)
	}
	
	authMs = append(authMs, ssh.Password(sftpc.DownloaderConfig.Password))

	config := &ssh.ClientConfig{
		User:            sftpc.DownloaderConfig.Username,
		Auth:           authMs,
		Timeout:         10 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	config.Ciphers = append(config.Config.Ciphers, "aes128-gcm@openssh.com")

	addr := fmt.Sprintf("%s:%d", sftpc.DownloaderConfig.Host, sftpc.DownloaderConfig.Port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}

	sftpc.sftpClient = client
	
	sftpc.DownloaderConfig.IsConnectedToServer = true

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

func (sftpc *SFTPClient) setFullLocalStagingAndRemotePath() {
	//set configured remote jailed directory + actual working directory
	wd, err := sftpc.sftpClient.Getwd()
	if err != nil {
		sftpc.DownloaderConfig.fullRemoteDir = sftpc.DownloaderConfig.RemoteDirectory
	} else {
		if sftpc.DownloaderConfig.RemoteDirectory != "" {
			sftpc.DownloaderConfig.fullRemoteDir = filepath.Join(wd, sftpc.DownloaderConfig.RemoteDirectory )
		}
	}

	sftpc.DownloaderConfig.fullLocalStagingDir = filepath.Join(sftpc.DownloaderConfig.LocalStagingBaseDirectory, sftpc.DownloaderConfig.LocalStagingDirectory)
}

func (sftpc *SFTPClient) newPublicKeyAuthMethod() (ssh.AuthMethod, error) {
	pemBytes, err := ioutil.ReadFile(sftpc.DownloaderConfig.PrivateKeyPath)
    if err != nil {
        return nil, err
    }
    signer, err := ssh.ParsePrivateKey(pemBytes)
    if err != nil {
		return nil, err
    }

	return ssh.PublicKeys(signer), nil
}