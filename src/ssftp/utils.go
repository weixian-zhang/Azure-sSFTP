package main

import (
	"encoding/json"
	"os"
	"runtime"
)

// func moveFile(oldloc string, newloc string) (error) {
// 	err := os.Rename(oldloc, newloc)
// 	logclient.ErrIf(err)
// 	return err
// }

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

func isDirExist(path string) (bool) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
	   return false
	} else {
		return true
	}
}

func isFileExist(filePath string) (bool) {
	if _, err := os.Stat(filePath); err == nil {
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

func ToJsonString(v interface{}) (string) {
	if v == nil {
		return "{}"
	}

	b, _ := json.Marshal(v)

	return string(b)
}

func isWindows() (bool) {
	if runtime.GOOS == "windows" {
		return true
	} else {
		return false
	}
}