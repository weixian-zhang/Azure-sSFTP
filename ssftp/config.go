package main

import (
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	stagingPath string
	cleanPath string
	quarantinePath string
	errorPath string
}

func NewConfig() (Config, error) {
	conf := Config{
		stagingPath: os.Getenv("stagingPath"),
		cleanPath: os.Getenv("cleanPath"),
		quarantinePath: os.Getenv("quarantinePath"),
		errorPath: "",
	}
	conf.errorPath = filepath.Join(conf.stagingPath, "error")

	if conf.stagingPath == "" || conf.cleanPath == "" || conf.quarantinePath == "" {
		err := errors.New("Environment variables missing for stagingPath, cleanPath or quarantinePath")
		logclient.ErrIf(err)
		return conf, err
	}

	return conf, nil
}