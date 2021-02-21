package main

import (
	"fmt"
	"time"
	"os"
	"io"
	"github.com/radovskyb/watcher"
	"path/filepath"
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
	watcher *watcher.Watcher
	configWatcher *watcher.Watcher
	fileCreateEvent chan File
	fileMoved chan FileMovedByStatus
}

func NewFileWatcher(confsvc *ConfigService) (FileWatcher) { 

	w := watcher.New()
	w.FilterOps(watcher.Create, watcher.Write, watcher.Rename)

	aerr := w.AddRecursive(confsvc.config.StagingPath)
	logclient.ErrIf(aerr)
	
	cw := watcher.New()
	cw.FilterOps(watcher.Write)
	conferr := cw.AddRecursive(confsvc.getYamlConfgPath())
	logclient.ErrIf(conferr)

	fileChan := make(chan File)
	return FileWatcher{
		watcher: w,
		configWatcher: cw,
		confsvc: confsvc,
		fileCreateEvent: fileChan,
	}
}

func (fw FileWatcher) startWatchConfigFileChange() {

	go fw.registerConfigFileChangeEvent()

	serr := fw.configWatcher.Start(time.Millisecond * 300)
	logclient.ErrIf(serr)
}

//startWatch goes into a control loop to continuously watch for newly created files
func (fw FileWatcher) startWatch(dirPathToWatch string) {

	logclient.Info(fmt.Sprintf("ssftp started watching directory: %s", dirPathToWatch))

	go fw.registerFileWatchEvents()
	
	serr := fw.watcher.Start(time.Millisecond * 100)
	logclient.ErrIf(serr)
}

func (fw FileWatcher) registerFileWatchEvents() {

	for {

		select {

			case err := <- fw.watcher.Error:
				logclient.ErrIf(err)

				
			case event := <- fw.watcher.Event:

				if event.IsDir() {
					continue
				}

				//only watch for create event
				if event.Op == watcher.Create || event.Op == watcher.Rename || event.Op == watcher.Write  {

					time.Sleep(1 * time.Second)

					logclient.Infof("File watch on file: %s", event.Name())

					fileOnWatch := File{
						Path: filepath.FromSlash(event.Path),
						Name: event.Name(), 
						Size: event.Size(),
						Operation: event.Op.String(),
						TimeCreated: (time.Now()).Format(time.ANSIC),
					}

					fw.fileCreateEvent <-fileOnWatch //notifies overlord to scan file

					<-fw.fileMoved // blocks, continue only after previous file scan done

					logclient.Infof("File watch unblocked for %s, continue with next file", fileOnWatch.Path)
					
				}
		}
	}
}

func (fw FileWatcher) registerConfigFileChangeEvent() {
	for {
		select {
			case err := <- fw.configWatcher.Error:
				logclient.ErrIf(err)

			case event := <- fw.configWatcher.Event:

				if SystemConfigFileName ==  event.Name() {
					logclient.Infof("sSFTP config file %s change detected", SystemConfigPath)

					fw.confsvc.LoadYamlConfig()
				}

		}
	}
}

//moveFileBetweenDrives also creates subfolders following staging/../.. if any
func (fw FileWatcher) moveFileBetweenDrives(srcPath string, destPath string) (error) {

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

