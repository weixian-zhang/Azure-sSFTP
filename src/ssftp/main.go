
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

	logclient.InitLogDests(&confsvc)

	ug := user.NewUserGov(confsvc.config.Users)

	sftpsvc := NewSFTPService(&confsvc, &ug)
	go sftpsvc.Start()
	
	ol, err := NewOverlord(&confsvc, &ug)
	logclient.ErrIf(err)

	exit := make(chan bool)

	ol.Start(exit)

	<- exit
}