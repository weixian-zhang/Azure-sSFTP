package main

import (
	"os"
	"fmt"
)

type StdClient struct {}

func (stdl StdClient) Info(msg string) {
	fmt.Fprintf(os.Stdout, msg)
}

func (stdl StdClient) Err(err error) {
	fmt.Fprintf(os.Stderr, err.Error() )
}

