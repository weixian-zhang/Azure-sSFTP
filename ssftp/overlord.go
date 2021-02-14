package main

import (
	"time"
	"path/filepath"
)

type Overlord struct {
	config      Config
	clamav      ClamAv
	fileWatcher FileWatcher
	fileMoved   chan FileMovedByStatus
}

type FileMovedByStatus struct {
	Path string
}

func NewOverlord(conf Config) (Overlord, error) {

	// afs :=  NewAzFileClient(conf)
	// fserr := afs.createFileShares()
	// if isErr(fserr) {
	// 	return Overlord{}, err
	// }

	clamav := NewClamAvClient()

	proceed := make(chan bool)

	go proceedOnClamdConnect(clamav, proceed)

	<- proceed //block until ssftp able to connect to Clamd on tcp://localhost:3310

	onFileMoved := make(chan FileMovedByStatus)

	fw := NewFileWatcher()
	fw.fileMoved = onFileMoved

	return Overlord{
		config: conf,
		clamav: clamav,
		fileWatcher: fw,
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

	overlord.fileWatcher.startWatch(overlord.config.stagingPath)

	//TODO: azfile move clean file to cleanpath and virus file to quarantine path
}

func (overlord Overlord) moveFileByStatus(scanR ClamAvScanResult) {

	cleanPath := filepath.Join(overlord.config.cleanPath, scanR.fileName)
	quarantinePath := filepath.Join(overlord.config.quarantinePath, scanR.fileName)

	if scanR.Status == OK {

		err := moveFile(scanR.filePath, cleanPath)
		if logclient.ErrIf(err) {
			return
		}

		logclient.Infof("moving clean file %s to %s", scanR.fileName, cleanPath)

	} else if scanR.Status == Virus {

		err := moveFile(scanR.filePath, quarantinePath)
		if logclient.ErrIf(err) {
			return
		}

		logclient.Infof("Virus found on %s, moving file to %s", scanR.fileName, quarantinePath)

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

