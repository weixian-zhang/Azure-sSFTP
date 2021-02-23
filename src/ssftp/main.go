
package main

import (
	"github.com/weixian-zhang/ssftp/user"
)

var logclient LogClient

func main() {

	logclient = NewBasicLogClient()

	confsvc := NewConfigService()
	configLoaded := confsvc.LoadYamlConfig()

	<- configLoaded

	ug := user.NewUserGov(confsvc.config.Users)

	//routes := ug.createSftpSvcRoutes()
	sftpsvc := NewSFTPService(&confsvc, ug)
	sftpsvc.Start()
	
	logclient.InitLogDests(*confsvc.config)
	logclient.Info("sSFTP started...")

	
	//ol, err := NewOverlord(&confsvc)
	//logclient.ErrIf(err)

	exit := make(chan bool)

	//ol.startWork(exit)

	<- exit
}