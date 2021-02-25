
package main

import (
	"fmt"
	"time"
	"os"
	"io"
	"github.com/radovskyb/watcher"
	"path/filepath"
	"github.com/weixian-zhang/ssftp/user"
	"strings"
)

type File struct {
	Name string
	Path string
	Size int64
	Operation string
	TimeCreated string
}

type FileWatcher struct {
	confsvc *ConfigService
	usergov  *user.UserGov
	watcher *watcher.Watcher
	configWatcher *watcher.Watcher
	ScanDone		chan bool
	fileCreateChangeEvent chan File
	fileMovedEvent chan FileMoveChanContext
}

type FileMoveChanContext struct {
	IsVirusFound bool
	DestPath string
}

func NewFileWatcher(confsvc *ConfigService, usrgov  *user.UserGov, scanDone chan bool) (FileWatcher) { 

	w := watcher.New()
	w.FilterOps(watcher.Create, watcher.Write, watcher.Rename)

	aerr := w.AddRecursive(confsvc.config.StagingPath)
	logclient.ErrIf(aerr)
	
	cw := watcher.New()
	cw.FilterOps(watcher.Write)
	conferr := cw.AddRecursive(confsvc.getYamlConfgPath())
	logclient.ErrIf(conferr)

	return FileWatcher{
		watcher: w,
		configWatcher: cw,
		confsvc: confsvc,
		usergov: usrgov,
		fileCreateChangeEvent:  make(chan File),
		fileMovedEvent: make(chan FileMoveChanContext),
		ScanDone: scanDone,
	}
}

func (fw *FileWatcher) startWatchConfigFileChange() {

	go fw.registerConfigFileChangeEvent()

	serr := fw.configWatcher.Start(time.Millisecond * 300)
	logclient.ErrIf(serr)
}

//startWatch goes into a control loop to continuously watch for newly created files
func (fw *FileWatcher) startStagingDirWatch(dirPathToWatch string) {

	logclient.Info(fmt.Sprintf("ssftp started watching directory: %s", dirPathToWatch))

	go fw.registerFileWatchEvents()
	
	serr := fw.watcher.Start(time.Millisecond * 100)
	logclient.ErrIf(serr)
}

func (fw *FileWatcher) registerFileWatchEvents() {

	for {

		select {

			case err := <- fw.watcher.Error:
				logclient.ErrIf(err)

				
			case event := <- fw.watcher.Event:

				if event.IsDir() {
					continue
				}

				if event.Op == watcher.Create || event.Op == watcher.Write  {

					time.Sleep(1 * time.Second)

					logclient.Infof("Filewatcher: file upload/write detected: %s", event.Name())

					fileOnWatch := File{
						Path: filepath.FromSlash(event.Path),
						Name: event.Name(), 
						Size: event.Size(),
						Operation: event.Op.String(),
						TimeCreated: (time.Now()).Format(time.ANSIC),
					}

					fw.fileCreateChangeEvent <-fileOnWatch //notifies overlord to scan file

					logclient.Infof("Filewatcher blocks for %s", fileOnWatch.Path)

					<- fw.ScanDone // blocks, continue only after previous file scan done

					logclient.Infof("Filewatcher unblocked for %s, continue with next file", fileOnWatch.Path)
					
				}
		}
	}
}

func (fw *FileWatcher) registerConfigFileChangeEvent() {
	for {
		select {
			case err := <- fw.configWatcher.Error:
				logclient.ErrIf(err)

			case event := <- fw.configWatcher.Event:

				if event.IsDir() {
					continue
				}

				if event.Op == watcher.Create || event.Op == watcher.Write  {

					if SystemConfigFileName ==  event.Name() {

						logclient.Infof("Config file %s change detected", SystemConfigPath)
	
						loaded := fw.confsvc.LoadYamlConfig()

						config := <- loaded

						logclient.Infof("Config file loaded successfully")

						fw.usergov.SetUsers(config.Users)
					}
				}
		}
	}
}

func (fw *FileWatcher) moveFileByStatus(scanR ClamAvScanResult)  {

	//replace "staging" folder path with new Clean and Quarantine path so when file is moved to either
	//clean/quarantine, the sub-folder structure remains the same as staging.
	//e.g: Staging:/mnt/ssftp/'staging'/userB/sub = Clean:/mnt/ssftp/'clean'/userB/sub or Quarantine:/mnt/ssftp/'quarantine'/userB/sub
	cleanPath := strings.Replace(scanR.filePath, fw.confsvc.config.StagingPath, fw.confsvc.config.CleanPath, -1)
	quarantinePath := strings.Replace(scanR.filePath, fw.confsvc.config.StagingPath, fw.confsvc.config.QuarantinePath, -1)

	hasVirus := false
	destPath := cleanPath
	
	if scanR.Status == Virus {
		hasVirus = true
		destPath = quarantinePath
	} 

	if !hasVirus {

		err := fw.moveFileBetweenDrives(scanR.filePath,cleanPath)
		if err != nil {
			return
		}

		logclient.Infof("File %q is clean moving file to %q", scanR.fileName, cleanPath)

	} else {

		err := fw.moveFileBetweenDrives(scanR.filePath, quarantinePath)
		if err != nil {
			return
		}

		logclient.Infof("Virus found in file %q, moving file to quarantine %q", scanR.fileName, quarantinePath)

		//fw.httpClient.callWebhook(scanR.fileName, quarantinePath)

		//TODO: trigger webhook
	}

	fw.fileMovedEvent <- FileMoveChanContext{
		IsVirusFound: hasVirus,
		DestPath: destPath,
	}
}

//moveFileBetweenDrives also creates subfolders following staging/../.. if any
func (fw *FileWatcher) moveFileBetweenDrives(srcPath string, destPath string) (error) {

	srcFile, err := os.Open(srcPath)
    if logclient.ErrIf(err) {
		return err
	}

	//creates all subfolders following staging/.../... if any
	destDirPathonly := filepath.Dir(destPath)
	if err := os.MkdirAll(destDirPathonly, os.ModePerm); os.IsExist(err) {
		logclient.Infof("Path exist %s", destDirPathonly)
	}

    destFile, err := os.Create(destPath)
    if logclient.ErrIf(err) {
		srcFile.Close()
		return err
	}

    defer destFile.Close()

    _, err = io.Copy(destFile, srcFile)
    srcFile.Close()
    if err != nil {
        logclient.ErrIf(err)
		return err
    }

    // The copy was successful, so now delete the original file
    err = os.Remove(srcPath)
    if logclient.ErrIf(err) {
		return err
	}

	return nil
}

