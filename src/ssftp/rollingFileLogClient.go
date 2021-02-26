

package main

import (
	"errors"
	"log"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

//https://github.com/natefinch/lumberjack

type RollingFileLogClient struct {
	confsvc *ConfigService
	errorWriter *log.Logger
	infoWriter *log.Logger
}

func NewRollingFileLogClient(confsvc *ConfigService) (RollingFileLogClient) {

	infoFileName := "ssftp-info.log"
	errorFileName := "ssftp-error.log"

	logPath := confsvc.getLogDestProp("file", "path")
	if logPath == "" {
		logclient.ErrIf(errors.New("LogDest file path not found"))
		return RollingFileLogClient{}
	}
	
	infow := log.New(&lumberjack.Logger{
		Filename:   filepath.Join(logPath, infoFileName),
		MaxSize:    10, // megabytes
		MaxBackups: 0,
		MaxAge:     1, //days
		LocalTime: true,
		Compress:   false, // disabled by default
	}, "", 0)

	errorw := log.New(&lumberjack.Logger{
		Filename:   filepath.Join(logPath, errorFileName),
		MaxSize:    10, // megabytes
		MaxBackups: 0,
		MaxAge:     1, //days
		LocalTime: true,
		Compress:   false, // disabled by default
	}, "", 0)

	return RollingFileLogClient{
		confsvc: confsvc,
		errorWriter: errorw,
		infoWriter: infow,
	}
}

func (rfc RollingFileLogClient) Info(msg string) {
	rfc.infoWriter.Println(msg)
}

func (rfc RollingFileLogClient) Err(err error) {
	rfc.errorWriter.Println((err.Error()))
}

