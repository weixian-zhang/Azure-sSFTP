package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type HttpClient struct {
	config Config
}

type VirusDetectedWebhookPostData struct {
	FileName string			`json:"fileName"`
	MountFilePath string	`json:"mountFilePath"`
	TimeGenerated string	`json:"timeGenerated"`
}

func NewHttpClient(conf Config) (HttpClient) {
	return HttpClient{
		config: conf,
	}
}

func (hc HttpClient) callWebhook(virusDetectedFileName string, mountFilePath string) {

	b, jerr := json.Marshal(VirusDetectedWebhookPostData{
		FileName: virusDetectedFileName,
		MountFilePath: mountFilePath,
		TimeGenerated: (time.Now()).Format(time.ANSIC),
	})

	if !logclient.ErrIf(jerr) {

		if isValidUrl(hc.config.VirusFoundWebhookUrl) {

			logclient.Infof("Calling webhook %s with params %s", hc.config.VirusFoundWebhookUrl, string(b))
			resp, herr := http.Post(hc.config.VirusFoundWebhookUrl, "application/json", bytes.NewBuffer(b) )
			
			if !logclient.ErrIf(herr) {
				logclient.Infof("Webhook invokation status %s", resp.Status)
			}

		} else {
			logclient.ErrIf(errors.New("Webhook Url is invalid"))
		}
	}
}