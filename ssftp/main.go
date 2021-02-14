package main

import (
	//log "github.com/sirupsen/logrus"
	"fmt"
)

var logclient LogClient

func main() {

	conf, err := NewConfig()
	if isErr(err) {
		fmt.Println(fmt.Sprintf("sSFTP initialized config"))
	}
	
	logclient = NewLogClient(conf)

	logclient.Info("sSFTP started")
	
	ol, err := NewOverlord(conf)
	logclient.ErrIf(err)

	exit := make(chan bool)

	ol.startWork(exit)

	<- exit
}