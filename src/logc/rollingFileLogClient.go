

package logc

import (
	"log"
	"path/filepath"
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
)

//https://github.com/natefinch/lumberjack

type RollingFileLogClient struct {
	logPath string
	errorWriter *log.Logger
	infoWriter *log.Logger
}

func NewRollingFileLogClient(logPath string) (RollingFileLogClient) {

	infoFileName := "ssftp-info.log"
	errorFileName := "ssftp-error.log"

	if logPath == "" {
		fmt.Errorf("RollingFileLogClient - error whle initializing flat file client, log path not found")
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
		logPath: logPath,
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

