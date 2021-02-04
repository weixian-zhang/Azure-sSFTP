package main

import (
	"os"
)

type StdClient struct {}

func (stdl StdClient) Info(msg string) {
	os.Stdout.Write([]byte(msg))
}

func (stdl StdClient) Err(err error) {
	os.Stderr.Write([]byte(err.Error()))
}

