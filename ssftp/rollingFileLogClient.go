package main

import (
	"log"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

//https://github.com/natefinch/lumberjack

type RollingFileLogClient struct {
	config Config
	errorWriter *log.Logger
	infoWriter *log.Logger
}

func NewRollingFileLogClient(conf Config) (RollingFileLogClient) {

	infoFileName := "ssftp-info.log"
	errorFileName := "ssftp-error.log"

	infow := log.New(&lumberjack.Logger{
		Filename:   filepath.Join(conf.LogPath, infoFileName),
		MaxSize:    10, // megabytes
		MaxBackups: 0,
		MaxAge:     1, //days
		LocalTime: true,
		Compress:   false, // disabled by default
	}, "", 0)

	errorw := log.New(&lumberjack.Logger{
		Filename:   filepath.Join(conf.LogPath, errorFileName),
		MaxSize:    10, // megabytes
		MaxBackups: 0,
		MaxAge:     1, //days
		LocalTime: true,
		Compress:   false, // disabled by default
	}, "", 0)

	return RollingFileLogClient{
		config: conf,
		errorWriter: errorw,
		infoWriter: infow,
	}
}

func (rfc RollingFileLogClient) Info(msg string) {
	rfc.infoWriter.Println(createLogMessage(msg))
}

func (rfc RollingFileLogClient) Err(err error) {
	rfc.errorWriter.Println((createLogMessage(err.Error())))
}

