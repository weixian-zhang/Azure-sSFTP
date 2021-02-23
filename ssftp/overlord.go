
package main

import (
	//"path/filepath"
	"strings"
	"time"
)

type Overlord struct {
	confsvc      *ConfigService
	clamav      ClamAv
	fileWatcher FileWatcher
	//sftpservice *SftpService
	usergov 	UserGov
	httpClient HttpClient
	fileMoved   chan FileMovedByStatus
}

type FileMovedByStatus struct {
	Path string
}

func NewOverlord(confsvc *ConfigService) (Overlord, error) {

	clamav := NewClamAvClient()

	proceed := make(chan bool)

	go proceedOnClamdConnect(clamav, proceed)

	<- proceed //block until ssftp able to connect to Clamd on tcp://localhost:3310

	httpClient := NewHttpClient(confsvc)

	onFileMoved := make(chan FileMovedByStatus)

	fw := NewFileWatcher(confsvc)
	fw.fileMoved = onFileMoved

	return Overlord{
		confsvc: confsvc,
		clamav: clamav,
		fileWatcher: fw,
		httpClient: httpClient,
		fileMoved: onFileMoved,
		//sftpservice: sftpsvc,
		//usergov: ug,
	}, nil
}

func (overlord Overlord) startWork(exit chan bool) {

	//overlord.sftpservice.Start()

	go func() {

		for {

			select {

			//case sftpFileUploaded := <- overlord.sftpservice.writeNotifications:

				//logclient.Infof("User %s uploads file %s", sftpFileUploaded.Username, sftpFileUploaded.Path)

				//go overlord.clamav.ScanFile(sftpFileUploaded.Path)

				// case fileCreated := <- overlord.fileWatcher.fileCreateEvent:

				// 	go overlord.clamav.ScanFile(fileCreated.Path)

				case scanR := <-overlord.clamav.scanEvent:

					overlord.moveFileByStatus(scanR)

				case <- exit:

					overlord.fileWatcher.watcher.Close()
					logclient.Info("Overlord exiting due to exit signal")
					
			}
		}
		
	}()

	//overlord.fileWatcher.startWatch(overlord.confsvc.config.StagingPath)
}

func (overlord Overlord) moveFileByStatus(scanR ClamAvScanResult) {

	//replace "staging" folder path with new Clean and Quarantine path so when file is moved to either
	//clean/quarantine, the sub-folder structure remains the same as staging.
	//e.g: Staging:/mnt/ssftp/'staging'/userB/sub = Clean:/mnt/ssftp/'clean'/userB/sub or Quarantine:/mnt/ssftp/'quarantine'/userB/sub
	cleanPath := strings.Replace(scanR.filePath, overlord.confsvc.config.StagingPath, overlord.confsvc.config.CleanPath, -1)
	quarantinePath := strings.Replace(scanR.filePath, overlord.confsvc.config.StagingPath, overlord.confsvc.config.QuarantinePath, -1)

	// dir, _ := filepath.Split(scanR.filePath)
	// paths := strings.Split(filepath.FromSlash(dir), "/")
	// usrPathonly := paths[3:]
	// newCleanPath := filepath.Join(overlord.config.CleanPath, strings.Join(usrPathonly, "/"))
	// newQuaPath := filepath.Join(overlord.config.QuarantinePath, strings.Join(usrPathonly, "/"))

	// cleanPath, cerr := filepath.Rel(newCleanPath, scanR.filePath)
	// if logclient.ErrIf(cerr) {
	// 	return
	// }
	// quarantinePath, qerr := filepath.Rel(newQuaPath, scanR.filePath)
	// if logclient.ErrIf(qerr) {
	// 	return
	// }

	if scanR.Status == OK {

		err := overlord.fileWatcher.moveFileBetweenDrives(scanR.filePath,cleanPath)
		if err != nil {
			return
		}

		logclient.Infof("File %q is clean moving file to %q", scanR.fileName, cleanPath)

	} else if scanR.Status == Virus {

		err := overlord.fileWatcher.moveFileBetweenDrives(scanR.filePath, quarantinePath)
		if err != nil {
			return
		}

		logclient.Infof("Virus found in file %q, moving file to quarantine %q", scanR.fileName, quarantinePath)

		overlord.httpClient.callWebhook(scanR.fileName, quarantinePath)

		//TODO: trigger webhook
	}

	overlord.fileMoved <- FileMovedByStatus{Path: scanR.filePath}
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

