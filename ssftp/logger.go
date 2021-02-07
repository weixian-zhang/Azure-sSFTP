package main

import (
	"fmt"
	"encoding/json"
	// "github.com/sirupsen/logrus"
	// log "github.com/sirupsen/logrus"
	
)

var logsinks []LogSink

type LogSink interface {
	Info(msg string)
	Err(err error)
}

type LogClient struct {}

type logrusHook struct{}

type IOWriteToString struct {}

//NewLogClient inits a list of supported log sinks and returns a "generic" LogClient.
//LogClient.Info() and Err() will log to all supported sinks.
func NewLogClient() (LogClient) {

	//initLogrusForStdClient()

	logsinks = make([]LogSink, 0)
	logsinks = append(logsinks, StdClient{})

	return LogClient{}
}

func (lc LogClient) Info(msg string) {
	logInfoToSinks(msg)
}

//Infof logs string message in fmt.Sprintf format
func (lc LogClient) Infof(msg string, val ...string) {
	logInfoToSinks(fmt.Sprintf(msg, val))
}

func (lc LogClient) ErrIf(err error) (bool) {
	if err != nil {
		logErrToSinks(err)
		return true
	} else {
		return false
	}
}

//InfoStruct marshals struct to json strings before logging to all sinks
func (lc LogClient) InfoStruct(p interface{}) {
	j, _ := json.Marshal(p)
	logInfoToSinks(string(j))
}

func logInfoToSinks(msg string) {
	for _, v := range logsinks {
		v.Info(msg)
	}
}

func logErrToSinks(err error) {
	for _, v := range logsinks {
		v.Err(err)
	}
}

// func (h *logrusHook) Levels() []logrus.Level {
//     return []logrus.Level{logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel, logrus.WarnLevel}
// }

// func (h *logrusHook) Fire(entry *logrus.Entry) error {

// 	msg, _ := entry.String()

// 	if entry.Level != logrus.ErrorLevel {
// 		logInfoToSinks(msg)
// 	} else {
// 		logErrToSinks(errors.New(msg))
// 	}

//     // logrus.Entry.log() is a non-pointer receiver function so it's goroutine safe to re-define *entry.Logger. The
//     // only race condition is between hooks since there is no locking. However .log() calls all hooks in series, not
//     // parallel. Therefore it should be ok to "duplicate" Logger and only change the Out field.
//     // loggerCopy := reflect.ValueOf(*entry.Logger).Interface().(logrus.Logger)
//     // entry.Logger = &loggerCopy
// 	// entry.Logger.Out = os.Stderr
	
	
//      return nil
// }

// func initLogrusForStdClient() {
// 	// Log as JSON instead of the default ASCII formatter.
// 	log.SetFormatter(&log.JSONFormatter{})

// 	// Output to stdout instead of the default stderr
// 	// Can be any io.Writer, see below for File example
// 	log.SetOutput(os.Stdout)

// 	log.AddHook(&logrusHook{})
  
// 	// Only log the warning severity or above.
// 	log.SetLevel(log.WarnLevel)
// }
