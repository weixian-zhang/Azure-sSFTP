package sftpclient

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"github.com/weixian-zhang/ssftp/logc"
	"github.com/weixian-zhang/ssftp/putty"
	"golang.org/x/crypto/ssh"
)

const ExtendedAttriPrefix = "user."

type SftpClient struct {
	DLConfig *DownloaderConfig
	UplConfig *UploaderConfig
	sftpClient *sftp.Client
	logclient *logc.LogClient
	uploadpaths UploadPaths
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

type UploaderConfig struct {
	UplName string
	Host string
	Port int
	Username string
	Password string
	PrivateKeyPath string
	PrivatekeyPassphrase string
	LocalCleanBaseDirectory string
	LocalRemoteUploadArchiveBasePath string
	LocalDirectoryToUpload string
	RemoteDirectory string
	OverrideRemoteExistingFile bool
}

type UploadPaths struct {
	fullLocalCleanUploadDir string				//clean dir path + configured local dir
	fullLocalFilePath string					//fullLocalCleanUploadDir + file name
	archiveBaseDir string						//found in config.go /mnt/ssfpt/clean/remoteupload-archive
	fullArchiveDir string						//RemoteUploadArchivePath + local clean dir
	fullArchiveWithSubDirsOnly string			//RemoteUploadArchivePath + local clean dir + sub dir difference
	fullArchiveFilePath string					//RemoteUploadArchivePath + local clean sub dirs + uploaded file name
	fullRemoteDir string						//remote jailed dir + configured remote dir
	fullRemoteFilePath string					//remote jailed dir + configured remote dir + file name to upload
	subDirsDifferenceOnly string				//sub dirs difference between local clean and archive
}

type StringArray []string

func NewSftpClient(downloaderConfig *DownloaderConfig, uploaderConf *UploaderConfig, logclient *logc.LogClient) *SftpClient {
	return &SftpClient{
		DLConfig: downloaderConfig,
		UplConfig: uploaderConf,
		sftpClient: nil,
		logclient: logclient,
	}
}

func (sftpc *SftpClient) DownloadFilesRecursive() (error) {

	sftpc.logclient.Infof("SftpClient Downloader %s - start seeking files in Sftp server %s@%s:%d", sftpc.DLConfig.DLName, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port)
	
	defer sftpc.sftpClient.Close()

	sftpc.setDownloaderFullLocalStagingAndRemotePath()

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
		localFile, err := sftpc.createLocalFile(localFullFilePath + ".download")
		
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

		//set ext attr "isdownloading" false
		defer sftpc.unMarkFileInDownloadingState(localFile.Name(), rmtFilePath)

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

func (sftpc *SftpClient) unMarkFileInDownloadingState(path string, remoteFilePath string) {
	markedFileName := strings.Replace(path, ".download", "", 1)
	err := os.Rename(path, markedFileName)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error marking file as download %s for %s@%s:%d-%s", err, sftpc.DLConfig.DLName, path, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port, remoteFilePath)
	}
}

func (sftpc *SftpClient) UploadFilesRecursive() (error) {
	
	sftpc.logclient.Infof("SftpClient Uploader %s - start seeking local files to upload %s@%s:%d", sftpc.UplConfig.UplName, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
	
	defer sftpc.sftpClient.Close()

	sftpc.setUploaderFullLocalCleanAndRemotePath()

	sftpc.ensureDir(sftpc.uploadpaths.archiveBaseDir)
	sftpc.ensureDir(sftpc.uploadpaths.fullLocalCleanUploadDir)

	//create remote dir if not exist
	err := sftpc.sftpClient.MkdirAll(sftpc.uploadpaths.fullRemoteDir)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error creating remote directory %s at %s@%s:%s", err, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullRemoteDir, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
		return err
	}

	sftpc.logclient.Infof("SftpClient Uploader %s - walking directory %s seeking files to upload", sftpc.UplConfig.UplName, sftpc.uploadpaths.fullLocalCleanUploadDir)

	var filesToUpload = make(StringArray, 0)

	werr := filepath.Walk(sftpc.uploadpaths.fullLocalCleanUploadDir, func(path string, info os.FileInfo, err error) error {

		if err != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error walking upload-clean-directory %s ", err, sftpc.UplConfig.UplName, path)
			return nil
		}

		if !info.IsDir() {

			sftpc.logclient.Infof("SftpClient Uploader %s - detected file %s", sftpc.UplConfig.UplName, path)

			filesToUpload = append(filesToUpload, path)
		}

		return nil
	})

 	if werr != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error walking local directory %s at %s@%s:%s", werr, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullLocalCleanUploadDir, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
		return err
	}

	if len(filesToUpload) == 0 {
		sftpc.logclient.Infof("SftpClient Uploader %s - no files detected for upload", sftpc.UplConfig.UplName)
		return nil
	}

	sftpc.logclient.Infof("SftpClient Uploader %s - gathered files %s for upload", sftpc.UplConfig.UplName, filesToUpload.toMultiline())

	for _, v := range filesToUpload {

		localFile, err := os.Open(v)

		if err != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error opening local file %s at %s@%s:%s", err, sftpc.UplConfig.UplName, v, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
			continue
		}

		defer localFile.Close()

		sftpc.setUploaderFilePaths(localFile.Name())

		_, err := sftpc.sftpClient.Stat(sftpc.uploadpaths.fullRemoteFilePath)

		_, cerr := sftpc.sftpClient.Create(sftpc.uploadpaths.fullRemoteFilePath)
		if cerr != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error creating remote file %s at %s@%s:%s", cerr, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullRemoteFilePath, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
			continue
		}

		rmtFile, oerr := sftpc.sftpClient.OpenFile(sftpc.uploadpaths.fullRemoteFilePath, (os.O_WRONLY|os.O_CREATE|os.O_TRUNC))
		if oerr != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error opening remote file %s at %s@%s:%s", oerr, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullRemoteFilePath, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
			continue
		}

		_, coerr := io.Copy(rmtFile, localFile)
		if coerr != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error uploading local %s to server %s at %s@%s:%s", coerr, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullLocalFilePath, sftpc.uploadpaths.fullRemoteFilePath, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
			continue
		}

		merr := sftpc.moveUploadedFileFromCleanToArchive()
		if merr != nil {
			sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error moving file %s to archive %s at %s@%s:%s", merr, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullLocalFilePath, sftpc.uploadpaths.fullArchiveFilePath, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
			continue
		}
	}

	return nil
}

func (sftpc *SftpClient) Connect(clientType string, clientName string, host string, port int, username string, password string, privateKeyPath string, privatekeyPassphrase string) (error) {

	authMs := make([]ssh.AuthMethod, 0)

	if privateKeyPath != "" {

		sftpc.logclient.Infof("SftpClient %s - loading private key from path %s", clientType, privateKeyPath)

		pkAuthMethod, err := sftpc.newPublicKeyAuthMethod(clientName, privateKeyPath, privatekeyPassphrase)
		if err != nil {
			sftpc.logclient.ErrIffmsg("SftpClient %s - error occur while reading private key file. Ignoring Private Key authn.", err, clientType)
		} else {
			authMs = append(authMs, pkAuthMethod)
		}
	}
	
	authMs = append(authMs, ssh.Password(password))

	config := &ssh.ClientConfig{
		User:           username,
		Auth:           authMs,
		Timeout:         5 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	sftpc.logclient.Infof("SftpClient %s - attempting to login to server %s@%s:%d", clientName, username, host, port)

	conn, err := ssh.Dial("tcp", addr, config)
	
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient %s - error connecting to server at %s@%s:%d", err, clientName,  username, host, port)
		return err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient %s - error creating new Sftpclient", err, clientName)
		return err
	}

	sftpc.logclient.Infof("SftpClient %s %s - successfully login to server %s@%s:%d",clientType, clientName, username, host, port)

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

func (sftpc *SftpClient) setDownloaderFullLocalStagingAndRemotePath() {

	sftpc.uploadpaths = UploadPaths{}

	//set configured remote jailed directory + configured working directory
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

func (sftpc *SftpClient) setUploaderFullLocalCleanAndRemotePath() {

	sftpc.uploadpaths.archiveBaseDir = sftpc.UplConfig.LocalRemoteUploadArchiveBasePath

	//set configured remote jailed directory + configured working directory
	wd, err := sftpc.sftpClient.Getwd()

	if err != nil {
		sftpc.uploadpaths.fullRemoteDir = sftpc.UplConfig.RemoteDirectory
	} else {
		if sftpc.UplConfig.RemoteDirectory != "" {
			sftpc.uploadpaths.fullRemoteDir = filepath.Join(wd, sftpc.UplConfig.RemoteDirectory)
		} else {
			sftpc.uploadpaths.fullRemoteDir = wd
		}
	}

	//set full local path
	sftpc.uploadpaths.fullLocalCleanUploadDir = filepath.Join(sftpc.UplConfig.LocalCleanBaseDirectory, sftpc.UplConfig.LocalDirectoryToUpload)
}

func (sftpc *SftpClient) setUploaderFilePaths(uploadFilePath string){

	sftpc.uploadpaths.fullLocalFilePath = ""
	sftpc.uploadpaths.subDirsDifferenceOnly = ""
	sftpc.uploadpaths.fullRemoteFilePath  = ""
	sftpc.uploadpaths.fullArchiveDir = ""
	sftpc.uploadpaths.fullArchiveFilePath = ""

	sftpc.uploadpaths.fullLocalFilePath = uploadFilePath

	dir, _ := filepath.Split(uploadFilePath)
	cleansed := filepath.Clean(dir)
	sftpc.uploadpaths.subDirsDifferenceOnly = strings.Replace(cleansed, sftpc.uploadpaths.fullLocalCleanUploadDir, "" , 1)

	localFileToUploadPath := uploadFilePath
	localFileToUploadPathNameOnly := filepath.Base(localFileToUploadPath)
	sftpc.uploadpaths.fullRemoteFilePath = filepath.Join(sftpc.uploadpaths.fullRemoteDir, localFileToUploadPathNameOnly)
	
	//set full local clean remote upload archive path
	sftpc.uploadpaths.fullArchiveDir = filepath.Join(sftpc.uploadpaths.archiveBaseDir , sftpc.UplConfig.LocalDirectoryToUpload, sftpc.uploadpaths.subDirsDifferenceOnly)

	sftpc.uploadpaths.fullArchiveFilePath  = filepath.Join(sftpc.uploadpaths.fullArchiveDir, localFileToUploadPathNameOnly)
}

func (sftpc *SftpClient) moveUploadedFileFromCleanToArchive() (error) {

	//ensure archive dir and sub dirs exist
	//sftpc.ensureDir(sftpc.uploadpaths.fullArchiveDir)
	sftpc.ensureDir(sftpc.uploadpaths.fullArchiveDir)

	//file has moved by other goroutine
	if !sftpc.isDirFileExist(sftpc.uploadpaths.fullLocalFilePath) {
		return nil
	}

	srcFile, err := os.Open(sftpc.uploadpaths.fullLocalFilePath)
    if os.IsNotExist(err) {
		sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error opening source file %s for moving on post upload", err, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullLocalFilePath)
		return err
	}
	defer srcFile.Close()

    destFile, err := os.Create(sftpc.uploadpaths.fullArchiveFilePath)
    if err != nil {
		srcFile.Close()
		sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error creating destination file %s for moving on post upload", err, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullArchiveFilePath)
		return err
	}
    defer destFile.Close()

    _, err = io.Copy(destFile, srcFile)
    srcFile.Close()
    if err != nil {
        sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error copying source file %s to destination file %s on post upload", err, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullLocalFilePath, sftpc.uploadpaths.fullArchiveFilePath)
		return err
    }

    // The copy was successful, so now delete the original file
    rerr := os.Remove(sftpc.uploadpaths.fullLocalFilePath)
    if rerr != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - error deleting source file %s after moving file to archive on post upload", rerr, sftpc.UplConfig.UplName, sftpc.uploadpaths.fullLocalFilePath)
		return rerr
	}

	return nil
}

func (sftpc *SftpClient) ensureDir(path string) {
	if _, serr := os.Stat(path); serr != nil {
		merr := os.MkdirAll(path, os.ModeDir)
		if merr != nil {
		  sftpc.logclient.ErrIffmsg("SftpClient Uploader %s - ensureDir throws error when creating nested directory %s", merr, sftpc.UplConfig.UplName, path)
		}
	  }
}

func (stra StringArray) toMultiline() (string) {
	var multil string
	for _, v := range stra {
		multil += v
		multil += "\n"
	}
	return multil
}

//cert authn
func (sftpc *SftpClient) newPublicKeyAuthMethod(clientName string, privateKeyPath string, privatekeyPassphrase string) (ssh.AuthMethod, error) {

	bytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient %s - error reading private key from %s", err, clientName, privateKeyPath)
		return nil, err
    }
	
	signer, err := sftpc.getSignerFromPrivateKey(clientName, bytes, privateKeyPath, privatekeyPassphrase)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient %s - error parsing pem key private key %s, try parsing in putty .ppk format", err, clientName, privateKeyPath)
		return nil, err
    }

	return ssh.PublicKeys(signer), nil
}

func (sftpc *SftpClient) getSignerFromPrivateKey(clientName string, pemBytes []byte, privateKeyPath string, privatekeyPassphrase string) (ssh.Signer, error) {

	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		sftpc.logclient.Infof("SftpClient %s - try to parse private key as PEM format failed for %s. Attempting Putty PPK key parsing", clientName, privateKeyPath)

		// parse PPK format (RSA, EC or DSA key)
		signerFrmPPK, ppkerr := sftpc.signerFromPPK(clientName, privateKeyPath, privatekeyPassphrase)

		if ppkerr != nil {
			return nil ,ppkerr
		} else {
			sftpc.logclient.Infof("SftpClient %s - parse private key Putty formatsuccessfully from %s", clientName, privateKeyPath)
			return signerFrmPPK, nil
		}
	}

	err := errors.New("Pem decode failed, no key found")
	// handle encrypted key
	if x509.IsEncryptedPEMBlock(pemBlock) {
		// decrypt PEM
		pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(privatekeyPassphrase))
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

func (sftpc *SftpClient) signerFromPPK(clientName string, privateKeyPath string, privatekeyPassphrase string) (ssh.Signer, error) {
	
	var privateKey interface{}

	// read the key
	puttyKey, err := putty.NewFromFile(privateKeyPath)
	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error parsing ppk private key %s", err, clientName, privateKeyPath)
		return nil, err
	}

	// parse putty key
	if puttyKey.Encryption != "none" {
		// If the key is encrypted, decrypt it
		privateKey, err = puttyKey.ParseRawPrivateKey([]byte(privatekeyPassphrase))
		if err != nil {
			privateKey = nil
			sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error decrypting ppk private key %s using passphrase %s", err, clientName, privateKeyPath, privatekeyPassphrase)
			return nil, err
		}
	} else {
		privateKey, err = puttyKey.ParseRawPrivateKey(nil)
		if err != nil {
			privateKey = nil
			sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error parsing raw ppk private key %s", err, clientName, privateKeyPath)
		}
	}

	signer, err := ssh.NewSignerFromKey(privateKey)

	if err != nil {
		sftpc.logclient.ErrIffmsg("SftpClient Downloader %s - error creating new signer from raw ppk private key %s", err, clientName, privateKeyPath)
		return nil, err
	}
	
	return signer, nil

}