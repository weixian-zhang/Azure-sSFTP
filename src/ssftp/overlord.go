
package main

import (
	"github.com/weixian-zhang/ssftp/user"
	"time"
)

type Overlord struct {
	confsvc      *ConfigService
	clamav      ClamAv
	fileWatcher FileWatcher
	//sftpservice *SftpService
	usergov 	*user.UserGov
	httpClient HttpClient
}


func NewOverlord(confsvc *ConfigService, usergov *user.UserGov) (Overlord, error) {

	clamav := NewClamAvClient()

	proceed := make(chan bool)
	go proceedOnClamdConnect(clamav, proceed)

	<- proceed //block until ssftp able to connect to Clamd on tcp://localhost:3310

	httpClient := NewHttpClient(confsvc)

	scanDone := make(chan bool)

	fw := NewFileWatcher(confsvc, usergov, scanDone)

	return Overlord{
		confsvc: confsvc,
		clamav: clamav,
		fileWatcher: fw,
		httpClient: httpClient,
		usergov: usergov,
		//sftpservice: sftpsvc,
		//usergov: ug,
	}, nil
}

func (ol *Overlord) Start(exit chan bool) {

	//overlord.sftpservice.Start()

	go ol.fileWatcher.startWatchConfigFileChange()

	go ol.fileWatcher.startStagingDirWatch(ol.confsvc.config.StagingPath)

	go func() {

		for {

			select {

			case fileCreateChange := <- ol.fileWatcher.fileCreateChangeEvent:

				logclient.Infof("sending file %s for scanning", fileCreateChange.Path)

				go ol.clamav.ScanFile(fileCreateChange.Path)


			case scanR := <-ol.clamav.scanEvent:

				ol.fileWatcher.ScanDone <- true

				logclient.Infof("Scanning done for file %s and virus found is %v", scanR.filePath, scanR.VirusFound)

				destPath := ol.fileWatcher.moveFileByStatus(scanR)

				if scanR.VirusFound {

					ol.httpClient.callVirusFoundWebhook(VirusDetectedWebhookData{
						FileName: scanR.fileName,
						FilePath: destPath,
						ScanMessage: scanR.Message,
						TimeGenerated: (time.Now()).Format(time.ANSIC),
					})
				}				

			case <- exit:

					ol.fileWatcher.watcher.Close()
					logclient.Info("Overlord exiting due to exit signal")
					
			}
		}
		
	}()

	//overlord.fileWatcher.startWatch(overlord.confsvc.config.StagingPath)
}



func proceedOnClamdConnect(clamav ClamAv, proceed chan bool) {
	for {
		_, err := clamav.PingClamd()

			if logclient.ErrIf(err) {
				time.Sleep(3 * time.Second)
			} else {
				logclient.Info("sSFTP connected to Clamd on tcp://localhost:3310")
				proceed <- true
				break
		}
	}
}

