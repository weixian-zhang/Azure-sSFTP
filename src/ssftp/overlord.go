package main

import (
	"sync"
	"time"
	"github.com/weixian-zhang/ssftp/sftpclient"
	"github.com/weixian-zhang/ssftp/user"
	"github.com/weixian-zhang/ssftp/webhook"
)

//time limit for upload file.
//If file in idling upload and mod time duration from now >= limit, trigger upload time out
//Also used by net.Conn on read write deadlines
const UploadTimeLimitMin int = 120

type Overlord struct {
	confsvc      *ConfigService
	clamav      ClamAv
	fileWatcher FileWatcher
	sftpservice *SFTPService
	usergov 	*user.UserGov
	httpClient webhook.HttpClient
	sftpclients []*sftpclient.SftpClient
}


func NewOverlord(confsvc *ConfigService, usergov *user.UserGov) (Overlord) {

	clamav := NewClamAvClient()

	//proceed := make(chan bool)
	go proceedOnClamdConnect(clamav) //, proceed)

	httpClient := webhook.NewHttpClient(&logclient)

	scanDone := make(chan bool)

	sftpsvc := NewSFTPService(confsvc, usergov)

	fw := NewFileWatcher(&sftpsvc, confsvc, usergov, scanDone)

	ol := Overlord{
		confsvc: confsvc,
		clamav: clamav,
		fileWatcher: fw,
		httpClient: httpClient,
		usergov: usergov,
		sftpservice: &sftpsvc,
	}

	ol.NewSFTPClients()

	return ol
}

func (ol *Overlord) Start(exit chan bool) {

	go func() {

		for {

			select {

				case scavengedFile := <- ol.fileWatcher.fileCreateChangeEvent:

					logclient.Infof("Overlord - sending file %s for scanning", scavengedFile.Path)

					if !ol.confsvc.config.EnableVirusScan {

						logclient.Infof("Overlord - Virus scan is disabled")

						ol.fileWatcher.ScanDone <- true

					} else {
						go ol.clamav.ScanFile(scavengedFile.Path)
					}

				case scanR := <-ol.clamav.scanEvent:

					if scanR.Error {
						logclient.Infof("Overlord - error while Clamd scans file %s, Error: %s",scanR.filePath, scanR.Message)
						ol.fileWatcher.ScanDone <- true
						break
					}

					logclient.Infof("Overlord - scanning done for file %s and virus = %v", scanR.filePath, scanR.VirusFound)

					destPath := ol.fileWatcher.moveFileByStatus(scanR)

					if scanR.VirusFound {

						url := ol.confsvc.getWebHook(VirusFoundWebook)
						ol.httpClient.CallVirusFoundWebhook(url, webhook.VirusDetectedWebhookData{
							FilePath: destPath,
							ScanMessage: scanR.Message,
							TimeGenerated: (time.Now()).Format(time.ANSIC),
						})
					}
					
					ol.fileWatcher.ScanDone <- true					
			}
		}
		
	}()

	go ol.StartSftpClientsDownloadFiles()

	go ol.StartSftpClientsUploadFiles()

	go ol.watchConfigFileChanges()

	go ol.fileWatcher.ScavengeUploadedFiles()

	ol.sftpservice.Start()
}

func (ol *Overlord) NewSFTPClients() {

	sftpcs := make([]*sftpclient.SftpClient, 0)

	for _, v := range ol.confsvc.config.ClientDownloaders {

		sftpcConf := &sftpclient.DownloaderConfig{
					DLName: v.Name,
					Host: v.Host,
					Port: v.Port,
					Username: v.Username,
					Password: v.Password,
					PrivateKeyPath: v.PrivatekeyPath,
					PrivatekeyPassphrase: v.PrivatekeyPassphrase,
					LocalStagingBaseDirectory: ol.confsvc.config.StagingPath,
					LocalStagingDirectory: v.LocalStagingDirectory,
					RemoteDirectory: v.RemoteDirectory,
					DeleteRemoteFileAfterDownload: v.DeleteRemoteFileAfterDownload,
					OverrideExistingFile: v.OverrideExistingFile,
				}

		sftpc := sftpclient.NewSftpClient(sftpcConf, nil, &logclient)

		sftpcs = append(sftpcs, sftpc)
	}

	for _, v := range ol.confsvc.config.ClientUploaders {

		sftpcConf := &sftpclient.UploaderConfig{
					UplName: v.Name,
					Host: v.Host,
					Port: v.Port,
					Username: v.Username,
					Password: v.Password,
					PrivateKeyPath: v.PrivatekeyPath,
					PrivatekeyPassphrase: v.PrivatekeyPassphrase,
					RemoteDirectory: v.RemoteDirectory,
					LocalCleanBaseDirectory: ol.confsvc.config.CleanPath,
					LocalRemoteUploadArchiveBasePath: ol.confsvc.config.LocalRemoteUploadArchiveBasePath,
					LocalDirectoryToUpload: v.LocalDirectoryToUpload,
					OverrideRemoteExistingFile: v.OverrideRemoteExistingFile,
				}

		sftpc := sftpclient.NewSftpClient(nil, sftpcConf, &logclient)

		sftpcs = append(sftpcs, sftpc)
	}

	ol.sftpclients = sftpcs
}

func (ol *Overlord) StartSftpClientsDownloadFiles() {
	
	logclient.Infof("Overlord - SFTP client downloader start connecting to SFTP servers")

	go func() {

		wg := sync.WaitGroup{}

		for {

			logclient.Info("Overlord - checking config invalidity before Downloaders execution")
			ol.confsvc.waitConfigValid()
			logclient.Info("Overlord - config is valid, begin Downloaders execution")

			if !ol.confsvc.config.EnableSftpClientDownloader {
				logclient.Infof("Overlord - EnableSftpClientDownloader is false, downloaders are disabled")
				time.Sleep(10 * time.Second)
				continue
			}

			for _, v := range ol.sftpclients {

				if v.DLConfig == nil {
					continue
				}

				err := v.Connect("Downloader", v.DLConfig.DLName, v.DLConfig.Host, v.DLConfig.Port, v.DLConfig.Username, v.DLConfig.Password, v.DLConfig.PrivateKeyPath, v.DLConfig.PrivatekeyPassphrase)
				if err != nil {
					logclient.ErrIffmsg("Overlord - error while SftpClient connecting to host: %s, port:%d", err, v.DLConfig.Host, v.DLConfig.Port)
					continue
				}

				go ol.DownloadFilesFromSFTPServer(v, &wg)
				wg.Add(1)
			}

			wg.Wait()

			time.Sleep(5 * time.Second)
		}
	}()
}

func (ol *Overlord) DownloadFilesFromSFTPServer(sftpc *sftpclient.SftpClient, wg *sync.WaitGroup) {

	err := sftpc.DownloadFilesRecursive()
	if err != nil {
		logclient.ErrIffmsg("Overlord - error while executing Sftp client file download to %s@%s:%s", err, sftpc.DLConfig.Username, sftpc.DLConfig.Host, sftpc.DLConfig.Port)
	}

	wg.Done()
}

func (ol *Overlord) StartSftpClientsUploadFiles() {
	logclient.Infof("Overlord - SFTP client uploader start connecting to SFTP servers")

	go func() {

		wg := sync.WaitGroup{}

		for {

			logclient.Info("Overlord - checking config invalidity before Uploaders execution")
			ol.confsvc.waitConfigValid()
			logclient.Info("Overlord - config is valid, begin Uploaders execution")

			if !ol.confsvc.config.EnableSftpClientUploader {
				logclient.Infof("Overlord - EnableSftpClientUploader is false, uploaders are disabled")
				time.Sleep(10 * time.Second)
				continue
			}

			//TODO loop thru uploaders only
			for _, v := range ol.sftpclients {

				if v.UplConfig == nil {
					continue
				}

				err := v.Connect("Uploader", v.UplConfig.UplName, v.UplConfig.Host, v.UplConfig.Port, v.UplConfig.Username, v.UplConfig.Password, v.UplConfig.PrivateKeyPath, v.UplConfig.PrivatekeyPassphrase)
				if err != nil {
					logclient.ErrIffmsg("Overlord - error while uploader connects to s@%s:%s", err, v.UplConfig.Username, v.UplConfig.Host, v.UplConfig.Port)
					continue
				}
				

				go ol.UploadFilesToSFTPServer(v, &wg)
				wg.Add(1)
			}

			wg.Wait()

			time.Sleep(5 * time.Second)

		}
	}()
}

func (ol *Overlord) UploadFilesToSFTPServer(sftpc *sftpclient.SftpClient, wg *sync.WaitGroup) {

	err := sftpc.UploadFilesRecursive()
	if err != nil {
		logclient.ErrIffmsg("Overlord - error while executing uploader on %s@%s:%s",err, sftpc.UplConfig.Username, sftpc.UplConfig.Host, sftpc.UplConfig.Port)
	}

	wg.Done()
}

func (ol *Overlord) watchConfigFileChanges() {

	configChange := make(chan bool)

	go ol.fileWatcher.startWatchConfigFileChange(configChange)

	for {
		select{
			//recreate Sftpclients on config change
			case <- configChange:

				ol.NewSFTPClients()

				ol.fileWatcher.usergov.SetUsers(ol.confsvc.config.Users)
		}
	}
}

func proceedOnClamdConnect(clamav ClamAv) {//, proceed chan bool) {
	for {
		_, err := clamav.PingClamd()

			if err != nil {
				logclient.Info("Overlord - sSFTP waiting for Clamd to be ready, connecting at tcp://localhost:3310")
				time.Sleep(3 * time.Second)
				continue
			} else {
				logclient.Info("Overlord - sSFTP connected to Clamd on tcp://localhost:3310")
				//proceed <- true
				break
		}
	}
}

