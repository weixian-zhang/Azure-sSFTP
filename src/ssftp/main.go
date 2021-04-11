
package main

import (
	"github.com/weixian-zhang/ssftp/user"
	"github.com/weixian-zhang/ssftp/logc"
)

var logclient logc.LogClient

func main() {

	logclient = logc.NewBasicStdoutLogClient()

	confsvc := NewConfigService()
	configValid := confsvc.LoadYamlConfig()

	<- configValid

	ug := user.NewUserGov(confsvc.config.Users)

	logPath := confsvc.GetLogDestProp("file", "path")

	logclient.InitLogDests(&logc.LogConfig{
		FlatFileLogPath: logPath,
	})

	ol := NewOverlord(&confsvc, &ug)

	exit := make(chan bool)

	ol.Start(exit)

	<- exit
}