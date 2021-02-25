
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

	//ol.fileWatcher.startStagingDirWatch(ol.confsvc.config.StagingPath)

	go func() {

		for {

			select {

			//case sftpFileUploaded := <- overlord.sftpservice.writeNotifications:

				//logclient.Infof("User %s uploads file %s", sftpFileUploaded.Username, sftpFileUploaded.Path)

				//go overlord.clamav.ScanFile(sftpFileUploaded.Path)

				// case filecc := <- overlord.fileWatcher.fileCreateChangeEvent:

				// 	go overlord.clamav.ScanFile(filecc.Path)

				// case scanR := <-ol.clamav.scanEvent:

				// 	ol.fileWatcher.moveFileByStatus(scanR)

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

