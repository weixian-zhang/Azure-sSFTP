package main

import (
	"os"
	"fmt"
)

type MountVolLogClient struct {
	logDir string
}

func (mvc MountVolLogClient) Info(msg string) {
	fmt.Fprintf(os.Stdout, msg)
}

func (mvc MountVolLogClient) Err(err error) {
	fmt.Fprintf(os.Stderr, err.Error() )
}

