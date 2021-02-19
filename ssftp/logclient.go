package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
	//"runtime"
)

var logsinks []LogSink

type LogSink interface {
	Info(msg string)
	Err(err error)
}

type LogClient struct {}

type LogMessage struct {
	TimeGenerated string
	//Caller string
	Category string
	Message string
}


//NewLogClient inits a list of supported log sinks and returns a "generic" LogClient.
//LogClient.Info() and Err() will log to all supported sinks.
func NewLogClient(conf Config) (LogClient) {

	//initLogrusForStdClient()
	
	rfLogClient := NewRollingFileLogClient(conf)
	stdLogClient := NewStdLogClient()

	logsinks = make([]LogSink, 0)
	logsinks = append(logsinks, stdLogClient)
	logsinks = append(logsinks, rfLogClient)

	return LogClient{}
}

func (lc LogClient) Info(msg string) {
	logInfoToSinks(msg)
}

//Infof logs string message in fmt.Sprintf format
func (lc LogClient) Infof(msgTemplate string, args ...interface{}) {
	logInfoToSinks(fmt.Sprintf(msgTemplate, args...))
}

func (lc LogClient) ErrIf(err error) (bool) {
	if err != nil {
		logErrToSinks(err)
		return true
	} else {
		return false
	}
}

func (lc LogClient) ErrIfm(msg string, err error) (bool) {
	if err != nil {
		logErrToSinks(errors.New(fmt.Sprintf(msg, err.Error())))
		return true
	} else {
		return false
	}
}

func (lc LogClient) ErrIffmsg(msgTemplate string, err error, args...string) (bool) {
	if err != nil {
		logErrToSinks(errors.New(fmt.Sprintf(msgTemplate, args) + "\nError: " + err.Error()))
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

func createLogMessage(val interface{}) (string) {

	t := time.Now()
	timegen := t.Format(time.ANSIC)

	lm := LogMessage {
		TimeGenerated: timegen,
		//Caller : getCaller(),
		Category: "Info",
		Message: "",
	}

	if w, ok := val.(string); ok {
		lm.Category = "Info"
		lm.Message = w
	} else if e, ok := val.(error); ok {
		lm.Category = "Error"
		lm.Message = e.Error()
	}

	b, _ := json.Marshal(lm)

	return string(b)
}

// func getCaller() (string) {
// 	var caller string = "?.go:0"
// 	_, file, line, ok := runtime.Caller(0)

// 	if ok {
// 		caller = file + ":" + string(line)
// 	}
// 	return caller
// }