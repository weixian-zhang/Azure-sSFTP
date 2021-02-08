package main

import (
	"errors"
	"os"
)

type Config struct {
	stagingPath string
	cleanPath string
	quarantinePath string
	errorPath string
	logPath string
	virusFoundWebhookUrl string
}

func NewConfig() (Config, error) {
	conf := Config{
		stagingPath: os.Getenv("stagingPath"),
		cleanPath: os.Getenv("cleanPath"),
		quarantinePath: os.Getenv("quarantinePath"),
		errorPath: os.Getenv("errorPath"),
		logPath: os.Getenv("logPath"),
		virusFoundWebhookUrl: os.Getenv("virusFoundWebhookUrl"),
	}

	if conf.stagingPath == "" || conf.cleanPath == "" || conf.quarantinePath == "" || conf.errorPath == "" {
		err := errors.New("Environment variables missing for stagingPath, cleanPath, quarantinePath or errorPath")
		logclient.ErrIf(err)
		return conf, err
	}

	return conf, nil
}