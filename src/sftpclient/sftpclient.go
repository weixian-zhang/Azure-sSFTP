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
	"github.com/weixian-zhang/ssftp/putty"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

type SftpClient struct {
	DLConfig *DownloaderConfig
	sftpClient *sftp.Client
	logclient *logc.LogClient
}

type DownloaderConfig struct {
	DLName string
	Host string
	Port int
	DeleteRemoteFileAfterDownload bool
	OverrideExistingFile bool
	Username string
	Password string
	PrivateKeyPath string
	PrivatekeyPassphrase string
	LocalStagingBaseDirectory string
	LocalStagingDirectory string
	RemoteDirectory string
	fullLocalStagingDir string
	fullRemoteDir string
	
}

func NewSftpClient(config *DownloaderConfig, logclient *logc.LogClient) *SftpClient {
	return &SftpClient{
		DLConfig: config,
		sftpClient: nil,
		logclient: logclient,
	}
}

func (sftpc *SftpClient) DownloadFilesRecursive() (error) {

	//https://sftptogo.com/blog/go-sftp/

	sftpc.logclient.Infof("SftpClient Downloader %s - start seeking files in Sftp server %s@%s:%d", sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port)
	
	defer sftpc.sftpClient.Close()

	sftpc.setFullLocalStagingAndRemotePath()

	sftpc.createLocalDir(sftpc.DLConfig.fullLocalStagingDir)
	
	walker :=  sftpc.sftpClient.Walk(sftpc.DLConfig.fullRemoteDir)

	for walker.Step() {
		if walker.Err() != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error while directory walking", walker.Err(), sftpc.DLConfig.DLName)
			continue
		}

		sftpc.logclient.Infof("SftpClient Downloader %s - walking remote directory %s", sftpc.DLConfig.DLName, walker.Path())

		if walker.Stat().IsDir() { //sync remote and local dir structure
			sftpc.logclient.Infof("SftpClient Downloader %s - No file detected at %s, continue walking", sftpc.DLConfig.DLName, walker.Path())
			continue
		}

		rmtFilePath := walker.Path()
		sftpc.logclient.Infof("SftpClient Downloader %s - detected remote file %s@%s:%d-%s", sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port, rmtFilePath)

		localFullFilePath := filepath.Join(sftpc.DLConfig.fullLocalStagingDir, filepath.Base(rmtFilePath))

		if !sftpc.DLConfig.OverrideExistingFile {
			sftpc.logclient.Infof("SftpClient Downloader %s - detected file exist locally %s, skipping this file", sftpc.DLConfig.DLName, localFullFilePath)
			return nil
		} else {
			sftpc.logclient.Infof("SftpClient Downloader %s - OverrideExistingFile flag true, overriding local file %s with %s@%s:%d-%s", sftpc.DLConfig.DLName, localFullFilePath, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port, rmtFilePath)
		}
		
		//open remote file for reading
		remoteFile, err := sftpc.sftpClient.OpenFile(rmtFilePath, (os.O_RDONLY))
		if err != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error opening remote file %s@%s:%d-%s", err, sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port, rmtFilePath)
			return err
		}
		defer remoteFile.Close()

		//create local file same name as remote file
		localFile, err := sftpc.createLocalFile(localFullFilePath)
		if err != nil {
			return err
		}
		defer localFile.Close()

		sftpc.logclient.Infof("SftpClient Downloader %s - downloading remote file %s@%s:%d-%s", sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port, rmtFilePath)

		_, cerr := io.Copy(localFile, remoteFile)

		if cerr != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error while downloading file from %s", err, rmtFilePath, sftpc.DLConfig.DLName)
			return err
		}

		sftpc.logclient.Infof("SftpClient Downloader %s - file downloaded successfully from %s@%s:%d-%s to local: %s", sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port, rmtFilePath, localFullFilePath)

		if sftpc.DLConfig.DeleteRemoteFileAfterDownload {

			sftpc.logclient.Infof("SftpClient Downloader %s - DeleteRemoteFileAfterDownload is true, deleting remote file %s from %s@%s:%d", sftpc.DLConfig.DLName, rmtFilePath, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port)

			err := sftpc.sftpClient.Remove(rmtFilePath)
				if err != nil {
				sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error deleting remote file %s@%s:%d-%s", err, sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port, rmtFilePath)
				return err
			}
		} else {
			sftpc.logclient.Infof("SftpClient Downloader %s - DeleteRemoteFileAfterDownload is false, skip deleting remote file %s from %s@%s:%d", sftpc.DLConfig.DLName, rmtFilePath, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port)
		}
	}

	sftpc.logclient.Infof("SftpClient Downloader %s - completed directory walking, disconnecting from server %s@%s:%d", sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port)

	return nil
}

func (sftpc *SftpClient) Connect() (error) {

	authMs := make([]ssh.AuthMethod, 0)

	pkAuthMethod, err := sftpc.newPublicKeyAuthMethod()
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error occur while reading private key file. Ignoring Private Key authn.", err, sftpc.DLConfig.DLName)
	} else {
		authMs = append(authMs, pkAuthMethod)
	}
	
	authMs = append(authMs, ssh.Password(sftpc.DLConfig.Password))

	config := &ssh.ClientConfig{
		User:           sftpc.DLConfig.Username,
		Auth:           authMs,
		Timeout:         5 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	//config.Ciphers = append(config.Config.Ciphers, "aes128-gcm@openssh.com")

	addr := fmt.Sprintf("%s:%d", sftpc.DLConfig.Host, sftpc.DLConfig.Port)

	sftpc.logclient.Infof("SftpClient Downloader %s - attempting to login to server %s@%s:%d", sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port)

	conn, err := ssh.Dial("tcp", addr, config)
	
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error while logging in server", err, sftpc.DLConfig.DLName)
		return err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error while creating new Sftpclient", err, sftpc.DLConfig.DLName)
		return err
	}

	sftpc.logclient.Infof("SftpClient Downloader %s - successfully login to server %s@%s:%d", sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port)

	sftpc.sftpClient = client

	return nil
}

func (sftpc *SftpClient) createLocalDir(dir string) (error) {
	if !sftpc.isDirFileExist(dir) {
		err := os.Mkdir(dir, 0755)

		if err != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error creating local directory %s in staging directory ", err, sftpc.DLConfig.DLName, dir)
			return err
		}
	}

	return nil
}

func (sftpc *SftpClient) createLocalFile(file string) (*os.File, error) {

	if !sftpc.isDirFileExist(file) {
		f, err := os.Create(file)
		if err != nil {
			return nil, err
		} else {
			return f, nil
		}
	}

	if sftpc.DLConfig.OverrideExistingFile {
		//remove existing
		err := os.Remove(file)
		if err != nil {
			return nil, err
		}

		//recreate new
		f, err := os.Create(file)
		if err != nil {
			return nil, err
		} else {
			return f, nil
		}
	} else {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		} else {
			return f, nil
		}
	}
}

func (sftpc *SftpClient) isDirFileExist(path string) (bool) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func (sftpc *SftpClient) setFullLocalStagingAndRemotePath() {
	//set configured remote jailed directory + actual working directory
	wd, err := sftpc.sftpClient.Getwd()
	if err != nil {
		sftpc.DLConfig.fullRemoteDir = sftpc.DLConfig.RemoteDirectory
	} else {
		if sftpc.DLConfig.RemoteDirectory != "" {
			sftpc.DLConfig.fullRemoteDir = filepath.Join(wd, sftpc.DLConfig.RemoteDirectory )
		} else {
			sftpc.DLConfig.fullRemoteDir = wd
		}
	}

	sftpc.DLConfig.fullLocalStagingDir = filepath.Join(sftpc.DLConfig.LocalStagingBaseDirectory, sftpc.DLConfig.LocalStagingDirectory)
}

func (sftpc *SftpClient) newPublicKeyAuthMethod() (ssh.AuthMethod, error) {

	bytes, err := ioutil.ReadFile(sftpc.DLConfig.PrivateKeyPath)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error reading private key from %s", err, sftpc.DLConfig.DLName, sftpc.DLConfig.PrivateKeyPath)
		return nil, err
    }
	
	signer, err := sftpc.getSignerFromPrivateKey(bytes)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error parsing pem key private key %s, try parsing in putty .ppk format", err, sftpc.DLConfig.DLName, sftpc.DLConfig.PrivateKeyPath)
		return nil, err
    }

	return ssh.PublicKeys(signer), nil
}

func (sftpc *SftpClient) getSignerFromPrivateKey(pemBytes []byte) (ssh.Signer, error) {

	// read pem block

	
	
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		sftpc.logclient.Infof("SftpClient Downloader %s - try to parse private key as PEM format failed for %s. Attempting Putty PPK key parsing", sftpc.DLConfig.DLName, sftpc.DLConfig.PrivateKeyPath)

		// parse PPK format (RSA, EC or DSA key)
		signerFrmPPK, ppkerr := sftpc.signerFromPPK()

		if ppkerr != nil {
			return nil ,ppkerr
		} else {
			return signerFrmPPK, nil
		}
	}

	err := errors.New("Pem decode failed, no key found")
	// handle encrypted key
	if x509.IsEncryptedPEMBlock(pemBlock) {
		// decrypt PEM
		pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(sftpc.DLConfig.PrivatekeyPassphrase))
		if err != nil {
			return nil, fmt.Errorf("Decrypting PEM block failed %v", err)
		}

		// parse PEM format (RSA, EC or DSA key)
		key, err := sftpc.parsePemBlock(pemBlock)
		if err != nil {
			return nil, err
		}

		// generate signer instance from key
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			return nil, fmt.Errorf("Creating signer from encrypted key failed %v", err)
		}

		return signer, nil
	} else {
		// generate signer instance from plain key
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing plain private key failed %v", err)
		}

		return signer, nil
	}
}

func (sftpc *SftpClient) parsePemBlock(block *pem.Block) (interface{}, error) {
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing PKCS private key failed %v", err)
		} else {
			return key, nil
		}
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing EC private key failed %v", err)
		} else {
			return key, nil
		}
	case "DSA PRIVATE KEY":
		key, err := ssh.ParseDSAPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing DSA private key failed %v", err)
		} else {
			return key, nil
		}
	default:
		return nil, fmt.Errorf("Parsing private key failed, unsupported key type %q", block.Type)
	}
}

func (sftpc *SftpClient) signerFromPPK() (ssh.Signer, error) {
	
	var privateKey interface{}

	// read the key
	puttyKey, err := putty.NewFromFile(sftpc.DLConfig.PrivateKeyPath)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error parsing ppk private key %s", err, sftpc.DLConfig.DLName, sftpc.DLConfig.PrivateKeyPath)
		return nil, err
	}

	// parse putty key
	if puttyKey.Encryption != "none" {
		// If the key is encrypted, decrypt it
		privateKey, err = puttyKey.ParseRawPrivateKey([]byte(sftpc.DLConfig.PrivatekeyPassphrase))
		if err != nil {
			privateKey = nil
			sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error decrypting ppk private key %s using passphrase %s", err, sftpc.DLConfig.DLName, sftpc.DLConfig.PrivateKeyPath, sftpc.DLConfig.PrivatekeyPassphrase)
			return nil, err
		}
	} else {
		privateKey, err = puttyKey.ParseRawPrivateKey(nil)
		if err != nil {
			privateKey = nil
			sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error parsing raw ppk private key %s", err, sftpc.DLConfig.DLName, sftpc.DLConfig.PrivateKeyPath)
		}
	}

	signer, err := ssh.NewSignerFromKey(privateKey)

	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error creating new signer from raw ppk private key %s", err, sftpc.DLConfig.DLName, sftpc.DLConfig.PrivateKeyPath)
		return nil, err
	}
	
	return signer, nil

}