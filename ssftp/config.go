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

	stagingFileShareName string
	cleanFileShareName string
	quarantineFileShareName string
	errorFileShareName string
	logFileShareName string
	
	virusFoundWebhookUrl string
	// azStorageName string
	// azStorageKey string
}

const stagingFileShareDefaultName = "ssftp-staging"
const cleanFileShareDefaultName = "ssftp-clean"
const quarantineFileShareDefaultName = "ssftp-quarantine"
const errorFileShareDefaultName = "ssftp-error"
const logFileShareDefaultName = "ssftp-log"

func NewConfig() (Config, error) {
	conf := Config{
		stagingPath: os.Getenv("stagingPath"),
		cleanPath: os.Getenv("cleanPath"),
		quarantinePath: os.Getenv("quarantinePath"),
		errorPath: os.Getenv("errorPath"),
		logPath: os.Getenv("logPath"),

		stagingFileShareName: os.Getenv("stagingFileShareName"),
		cleanFileShareName: os.Getenv("cleanFileShareName"),
		quarantineFileShareName: os.Getenv("quarantineFileShareName"),
		errorFileShareName: os.Getenv("errorFileShareName"),
		logFileShareName: os.Getenv("logFileShareName"),

		//azStorageName: os.Getenv("azStorageName"),
		//azStorageKey: os.Getenv("azStorageKey"),
		
		virusFoundWebhookUrl: os.Getenv("virusFoundWebhookUrl"),
	}

	if conf.stagingPath == "" || conf.cleanPath == "" || conf.quarantinePath == "" || conf.errorPath == "" {
		err := errors.New("Environment variables missing for stagingPath, cleanPath, quarantinePath or errorPath")
		logclient.ErrIf(err)
		return conf, err
	}

	if conf.stagingFileShareName == "" {
		conf.stagingFileShareName = stagingFileShareDefaultName
	}
	if conf.cleanFileShareName == "" {
		conf.cleanFileShareName = cleanFileShareDefaultName
	}
	if conf.quarantineFileShareName == "" {
		conf.quarantineFileShareName = quarantineFileShareDefaultName
	}
	if conf.errorFileShareName == "" {
		conf.errorFileShareName = errorFileShareDefaultName
	}
	if conf.logFileShareName == "" {
		conf.logFileShareName = logFileShareDefaultName
	}

	return conf, nil
}