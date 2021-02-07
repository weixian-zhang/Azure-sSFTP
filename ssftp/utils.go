package main

import (
	"os"
)

func moveFile(oldloc string, newloc string) (error) {
	err := os.Rename(oldloc, newloc)
	logclient.ErrIf(err)
	return err
}

func isDir(path string) (bool) {
	f, err := os.Stat(path) 

	if isErr(err) {
		return false
	}

	if f.Mode().IsDir() {
		return true
	} else {
		return false
	}
}

func isErr(err error) bool {
	if err != nil {
		return true
	} else {
		return false
	}
}