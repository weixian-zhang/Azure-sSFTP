package main

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func StartWatch(clamav ClamAv, dirPathToWatch string) {

	watcher, err := fsnotify.NewWatcher()
	logclient.ErrIf(err)
	defer watcher.Close()

	exit := make(chan bool) //never set it to exit

	go onWatcherEventFired(watcher, clamav)

	watcher.Add(dirPathToWatch)
	
	<- exit
}

func onWatcherEventFired(watcher *fsnotify.Watcher, clamav ClamAv) {

	for {

		select {

			case watcherEvent := <- watcher.Events:

				//only watch for create event
				if watcherEvent.Op == fsnotify.Create {
					
					scanR := make(chan ClamAvScanResult)

					filePath := filepath.FromSlash(watcherEvent.Name)

					go clamav.ScanFile(filePath, scanR)

					processScanResult(<- scanR)
				}
		}
	}
}

func processScanResult(result ClamAvScanResult) {

	//TODO: 

}

