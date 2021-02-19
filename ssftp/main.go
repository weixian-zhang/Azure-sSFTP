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
	logclient.Info("sSFTP started...")

	ug := NewUserGov(conf)

	routes := make([]Route, 0)
	routes = append(routes, Route{
		Username: "testuser", 
		Password: "tiger" })
	routes = append(routes, Route{
			Username: "testuser2", 
			Password: "lion" })

	sftpsvc := NewSftpService("", 22, conf.StagingPath, routes, nil, ug)
	sftpsvc.Start()
	
	
	// ol, err := NewOverlord(conf)
	// logclient.ErrIf(err)

	exit := make(chan bool)

	//ol.startWork(exit)

	<- exit
}