package main

import (
	"os"
	"fmt"
)

//https://github.com/natefinch/lumberjack

type RollingFileLogClient struct {
	logDir string
}

func (mvc RollingFileLogClient) Info(msg string) {
	fmt.Fprintf(os.Stdout, msg)
}

func (mvc RollingFileLogClient) Err(err error) {
	fmt.Fprintf(os.Stderr, err.Error() )
}

