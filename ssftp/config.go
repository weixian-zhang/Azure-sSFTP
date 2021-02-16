package main

import (
	"errors"
	"os"
	"fmt"
	"log"
)

type Config struct {
	StagingPath string				`json:"stagingPath"`
	CleanPath string				`json:"CleanPath"`
	QuarantinePath string			`json:"QuarantinePath"`
	ErrorPath string				`json:"ErrorPath"`
	LogPath string					`json:"LogPath"`

	// StagingFileShareName string		`json:"StagingFileShareName"`
	// CleanFileShareName string		`json:"CleanFileShareName"`
	// QuarantineFileShareName string	`json:"QuarantineFileShareName"`
	// ErrorFileShareName string		`json:"ErrorFileShareName"`
	// LogFileShareName string			`json:"LogFileShareName"`
	
	VirusFoundWebhookUrl string		`json:"VirusFoundWebhookUrl"`
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
		StagingPath: os.Getenv("stagingPath"),
		CleanPath: os.Getenv("cleanPath"),
		QuarantinePath: os.Getenv("quarantinePath"),
		ErrorPath: os.Getenv("errorPath"),
		LogPath: os.Getenv("logPath"),

		// StagingFileShareName: os.Getenv("stagingFileShareName"),
		// CleanFileShareName: os.Getenv("cleanFileShareName"),
		// QuarantineFileShareName: os.Getenv("quarantineFileShareName"),
		// ErrorFileShareName: os.Getenv("errorFileShareName"),
		// LogFileShareName: os.Getenv("logFileShareName"),

		VirusFoundWebhookUrl: os.Getenv("virusFoundWebhookUrl"),
	}

	if conf.StagingPath == "" || conf.CleanPath == "" || conf.QuarantinePath == "" || conf.ErrorPath == "" {
		err := errors.New("Environment variables missing for stagingPath, cleanPath, quarantinePath or errorPath")
		log.Fatalln(err)
		return conf, err
	}

	// if conf.StagingFileShareName == "" {
	// 	conf.StagingFileShareName = stagingFileShareDefaultName
	// }
	// if conf.CleanFileShareName == "" {
	// 	conf.CleanFileShareName = cleanFileShareDefaultName
	// }
	// if conf.QuarantineFileShareName == "" {
	// 	conf.QuarantineFileShareName = quarantineFileShareDefaultName
	// }
	// if conf.ErrorFileShareName == "" {
	// 	conf.ErrorFileShareName = errorFileShareDefaultName
	// }
	// if conf.LogFileShareName == "" {
	// 	conf.LogFileShareName = logFileShareDefaultName
	// }

	configJStr := ToJsonString(conf)
	log.Println(fmt.Sprintf("sSFTP initialized config: %s", configJStr))

	return conf, nil
}