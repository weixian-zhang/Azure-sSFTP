
package main

import (
	"github.com/weixian-zhang/ssftp/user"
	"github.com/weixian-zhang/ssftp/logc"
)

var logclient logc.LogClient

func main() {

	logclient = logc.NewBasicStdoutLogClient()

	confsvc := NewConfigService()
	configLoaded := confsvc.LoadYamlConfig()

	<- configLoaded

	logPath := confsvc.GetLogDestProp("file", "path")

	logclient.InitLogDests(&logc.LogConfig{
		FlatFileLogPath: logPath,
	})

	ug := user.NewUserGov(confsvc.config.Users)
	
	ol := NewOverlord(&confsvc, &ug)

	exit := make(chan bool)

	ol.Start(exit)

	<- exit
}