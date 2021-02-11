package main

import (
	"os"
	"fmt"
)

type StdClient struct {}

func (stdl StdClient) Info(msg string) {
	fmt.Fprintf(os.Stdout, createLogMessage(msg))
}

func (stdl StdClient) Err(err error) {
	fmt.Fprintf(os.Stderr, createLogMessage(err) )
}

