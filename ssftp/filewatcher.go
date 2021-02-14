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
	watcher *watcher.Watcher//*fsnotify.Watcher
	fileCreateEvent chan File
	fileMoved chan FileMovedByStatus
}

func NewFileWatcher() (FileWatcher) { 

	w := watcher.New()
	
	w.FilterOps(watcher.Create, watcher.Write, watcher.Rename)
	//w.SetMaxEvents(1)

	fileChan := make(chan File)
	return FileWatcher{
		watcher: w,
		fileCreateEvent: fileChan,
	}
}

//startWatch goes into a control loop to continuously watch for newly created files
func (fw FileWatcher) startWatch(dirPathToWatch string) {

	logclient.Info(fmt.Sprintf("ssftp started watching directory: %s", dirPathToWatch))

	go fw.registerFileWatchEvents()

	//path := filepath.FromSlash(dirPathToWatch)


	aerr := fw.watcher.AddRecursive(dirPathToWatch)
	logclient.ErrIf(aerr)

	
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

func (fw FileWatcher) moveFileBetweenDrives(srcPath string, destPath string) (error) {

	srcFile, err := os.Open(srcPath)
    if logclient.ErrIf(err) {
		return err
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

