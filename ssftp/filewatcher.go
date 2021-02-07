package main

import (
	"path/filepath"
	//"log"
	"github.com/fsnotify/fsnotify"
)

type File struct {
	Path string
	Operation string
}

type FileWatcher struct {
	watcher *fsnotify.Watcher
	fileCreateEvent chan File
}

func NewFileWatcher() (FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	logclient.ErrIf(err)

	fileChan := make(chan File)
	return FileWatcher{
		watcher: watcher,
		fileCreateEvent: fileChan,
	}, err
}

//startWatch goes into a control loop to continuously watch for newly created files
func (fw FileWatcher) startWatch(dirPathToWatch string) {

	//defer fw.watcher.Close()

	go fw.onWatcherEventFired()

	path := filepath.FromSlash(dirPathToWatch)

	err := fw.watcher.Add(path)
	logclient.ErrIf(err)

}

func (fw FileWatcher) onWatcherEventFired() {

	for {

		select {

			//case watchErr := <- fw.watcher.Errors:
				
			case watcherEvent := <- fw.watcher.Events:

				//only watch for create event
				if !isDir(watcherEvent.Name) && watcherEvent.Op == fsnotify.Create {
					
					fw.fileCreateEvent <- File{
						Path: watcherEvent.Name, 
						Operation: watcherEvent.Op.String(),
					}
				}
		}
	}
}

