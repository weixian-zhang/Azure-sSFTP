package main

import (
	"time"
	"path/filepath"
)

type Overlord struct {
	config      Config
	clamav      ClamAv
	fileWatcher FileWatcher
	httpClient HttpClient
	fileMoved   chan FileMovedByStatus
}

type FileMovedByStatus struct {
	Path string
}

func NewOverlord(conf Config) (Overlord, error) {

	clamav := NewClamAvClient()

	proceed := make(chan bool)

	go proceedOnClamdConnect(clamav, proceed)

	<- proceed //block until ssftp able to connect to Clamd on tcp://localhost:3310

	httpClient := NewHttpClient(conf)

	onFileMoved := make(chan FileMovedByStatus)

	fw := NewFileWatcher()
	fw.fileMoved = onFileMoved

	return Overlord{
		config: conf,
		clamav: clamav,
		fileWatcher: fw,
		httpClient: httpClient,
		fileMoved: onFileMoved,
	}, nil
}

func (overlord Overlord) startWork(exit chan bool) {

	go func() {

		for {

			select {

				case fileCreated := <- overlord.fileWatcher.fileCreateEvent:

					go overlord.clamav.ScanFile(fileCreated.Path)

				case scanR := <-overlord.clamav.scanEvent:

					overlord.moveFileByStatus(scanR)

				case <- exit:

					overlord.fileWatcher.watcher.Close()
					logclient.Info("Overlord exiting due to exit signal")
					
			}
		}
		
	}()

	overlord.fileWatcher.startWatch(overlord.config.StagingPath)
}

func (overlord Overlord) moveFileByStatus(scanR ClamAvScanResult) {

	cleanPath := filepath.Join(overlord.config.CleanPath, scanR.fileName)
	quarantinePath := filepath.Join(overlord.config.QuarantinePath, scanR.fileName)

	if scanR.Status == OK {

		err := overlord.fileWatcher.moveFileBetweenDrives(scanR.filePath, cleanPath)
		if err != nil {
			return
		}

		logclient.Infof("File %q is clean moving file to %q", scanR.fileName, cleanPath)

	} else if scanR.Status == Virus {

		err := overlord.fileWatcher.moveFileBetweenDrives(scanR.filePath, string(quarantinePath))
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

