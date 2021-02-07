package main

import (
	//log "github.com/sirupsen/logrus"
)

var logclient LogClient

func main() {
	
	logclient = NewLogClient()

	logclient.Info("sSFTP started")
	
	ol, err := NewOverlord()
	logclient.ErrIf(err)

	exit := make(chan bool)

	ol.startWork(exit)

	<- exit
}