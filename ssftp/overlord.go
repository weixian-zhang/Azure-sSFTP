package main

import (
	"os"
	"path/filepath"
)

type Overlord struct {
	config      Config
	clamav      ClamAv
	fileWatcher FileWatcher
	azfile      Azfile
}

func NewOverlord() (Overlord, error) {
	
	conf, err := NewConfig()
	if isErr(err) {
		return Overlord{}, err
	}


	clamav, cerr := NewClamAvClient()
	if isErr(cerr) {
		return Overlord{}, cerr
	}

	fw, ferr := NewFileWatcher()
	if isErr(ferr) {
		return Overlord{}, ferr
	}

	createErrorDirIfNotExist(conf.errorPath)

	return Overlord{
		config: conf,
		clamav: clamav,
		fileWatcher: fw,
	}, nil
}

func (overlord Overlord) startWork(exit chan bool) {

	go func() {

		overlord.fileWatcher.startWatch(overlord.config.stagingPath)

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

		logclient.Infof("moving file %s to %s", scanR.fileName, cleanPath)

	} else if scanR.Status == Virus {

		err := moveFile(scanR.filePath, quarantinePath)
		if logclient.ErrIf(err) {
			return
		}

	}
}

func createErrorDirIfNotExist(errorPath string) {
	if _, err := os.Stat(errorPath); os.IsNotExist(err) {
		os.Mkdir(errorPath, os.ModePerm)
	}
}

