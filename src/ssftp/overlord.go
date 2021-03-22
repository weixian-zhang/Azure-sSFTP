
package main

import (
	"github.com/weixian-zhang/ssftp/user"
	"github.com/weixian-zhang/ssftp/sftpclient"
	"time"
	"sync"
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
	httpClient HttpClient
	sftpclients []*sftpclient.SFTPClient
}


func NewOverlord(confsvc *ConfigService, usergov *user.UserGov) (Overlord) {

	clamav := NewClamAvClient()

	proceed := make(chan bool)
	go proceedOnClamdConnect(clamav, proceed)

	//<- proceed //block until ssftp able to connect to Clamd on tcp://localhost:3310

	httpClient := NewHttpClient(confsvc)

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

	go ol.StartSftpClientsDownloadFiles()

	go ol.fileWatcher.startWatchConfigFileChange()

	go ol.fileWatcher.ScavengeUploadedFiles()

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

						ol.httpClient.callVirusFoundWebhook(VirusDetectedWebhookData{
							FilePath: destPath,
							ScanMessage: scanR.Message,
							TimeGenerated: (time.Now()).Format(time.ANSIC),
						})
					}
					
					ol.fileWatcher.ScanDone <- true					
			}
		}
		
	}()

	ol.sftpservice.Start()
}

func (ol *Overlord) NewSFTPClients() {
	if len(ol.confsvc.config.SFTPClientConnectors) == 0 {
		return
	}

	sftpcs := make([]*sftpclient.SFTPClient, 0)

	for _, v := range ol.confsvc.config.SFTPClientConnectors {
		sftpc := sftpclient.NewSftpClient(v.Host, v.Port, v.Username, v.Password, v.PrivatekeyPath, v.LocalStagingDirectory, v.RemoteDirectory, v.DeleteRemoteFileAfterDownload, v.OverrideExistingFile, &logclient)

		sftpcs = append(sftpcs, sftpc)
	}

	ol.sftpclients = sftpcs
}

func (ol *Overlord) StartSftpClientsDownloadFiles() {
	
	logclient.Infof("Overlord - SFTP clients start connecting to SFTP servers")

	go func() {

		wg := sync.WaitGroup{}

		for {

			//TODO:
				//continuously connect/reconnect/ignore when connected
				//once connected, start downloading file for each sftp client

			for _, v := range ol.sftpclients {
				err := v.Connect()
				if err != nil {
					logclient.ErrIffmsg("Overlord - error while SftpClient connecting to host: %s, port:%d", err, v.Host, v.Port)
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

func (ol *Overlord) DownloadFilesFromSFTPServer(sftpc *sftpclient.SFTPClient, wg *sync.WaitGroup) {
	err := sftpc.DownloadFilesRecursive()
	if err != nil {
		logclient.ErrIffmsg("Overlord - error while executing Sftp client file download host: %s, port: %d",err, sftpc.Host, sftpc.Port )
	}

	wg.Done()
}

func proceedOnClamdConnect(clamav ClamAv, proceed chan bool) {
	for {
		_, err := clamav.PingClamd()

			if err != nil {
				logclient.Info("Overlord - sSFTP waiting for Clamd to be ready, connecting at tcp://localhost:3310")
				time.Sleep(3 * time.Second)
				continue
			} else {
				logclient.Info("Overlord - sSFTP connected to Clamd on tcp://localhost:3310")
				proceed <- true
				break
		}
	}
}

