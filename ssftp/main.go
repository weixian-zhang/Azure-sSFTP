package main

var logclient LogClient

func main() {

	logclient = NewBasicLogClient()

	confsvc := NewConfigService()
	configLoaded := confsvc.LoadYamlConfig()

	<- configLoaded
	
	logclient.InitLogDests(*confsvc.config)
	logclient.Info("sSFTP started...")

	ug := NewUserGov(*confsvc.config)

	routes := ug.createSftpSvcRoutes()

	sftpsvc := NewSftpService("", confsvc.config.SftpPort, confsvc.config.StagingPath, routes, nil, ug)
	sftpsvc.Start()
	
	
	ol, err := NewOverlord(&confsvc)
	logclient.ErrIf(err)

	exit := make(chan bool)

	ol.startWork(exit)

	<- exit
}