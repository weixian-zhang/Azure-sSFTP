
package logc

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type LogSink interface {
	Info(msg string)
	Err(err error)
}

type LogClient struct {
	logConfig *LogConfig
	logsinks []LogSink
}

type LogMessage struct {
	TimeGenerated string
	//Caller string
	Category string
	Message string
}

type LogConfig struct {
	FlatFileLogPath string
}


//NewLogClient inits a list of supported log sinks and returns a "generic" LogClient.
//LogClient.Info() and Err() will log to all supported sinks.
func NewBasicStdoutLogClient() (LogClient) {
	
	stdLogClient := NewStdLogClient()
	logsinks := make([]LogSink, 0)
	logsinks = append(logsinks, stdLogClient)

	return LogClient{
		//configsvc: &ConfigService{},
		logsinks: logsinks,
	}
}

func (lc *LogClient) InitLogDests(logConfig *LogConfig) {

	lc.logConfig = logConfig

	if lc.logConfig.FlatFileLogPath != "" {
		rfLogClient := NewRollingFileLogClient(lc.logConfig.FlatFileLogPath)
		lc.logsinks = append(lc.logsinks, rfLogClient)
	}
}

func (lc *LogClient) Info(msg string) {
	lc.logInfoToSinks(msg)
}

//Infof logs string message in fmt.Sprintf format
func (lc *LogClient) Infof(msgTemplate string, args ...interface{}) {
	lc.logInfoToSinks(fmt.Sprintf(msgTemplate, args...))
}

func (lc *LogClient) ErrIf(err error) (bool) {
	if err != nil {
		lc.logErrToSinks(err)
		return true
	} else {
		return false
	}
}

func (lc *LogClient) ErrIfm(msg string, err error) (bool) {
	if err != nil {
		lc.logErrToSinks(errors.New(fmt.Sprintf(msg, err.Error())))
		return true
	} else {
		return false
	}
}

func (lc *LogClient) Errmfo(msgTemplate string, args...interface{}) {
	err := fmt.Errorf(msgTemplate, args)
	lc.logErrToSinks(err)
}

func (lc *LogClient) ErrIffmsg(msgTemplate string, err error, args...interface{}) (bool) {
	if err != nil {
		lc.logErrToSinks(errors.New(fmt.Sprintf(msgTemplate, args) + "\nError: " + err.Error()))
		return true
	} else {
		return false
	}
}

//InfoStruct marshals struct to json strings before logging to all sinks
func (lc *LogClient) InfoStruct(p interface{}) {
	j, _ := json.Marshal(p)
	lc.logInfoToSinks(string(j))
}

func (lc *LogClient) logInfoToSinks(msg string) {
	for _, v := range lc.logsinks {
		newMsg := lc.createLogMessage(msg)
		v.Info(newMsg)
	}
}

func (lc *LogClient) logErrToSinks(err error) {
	for _, v := range lc.logsinks {
		newMsg := lc.createLogMessage(err)
		v.Err(errors.New(newMsg))
	}
}

func (lc *LogClient) createLogMessage(val interface{}) (string) {

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