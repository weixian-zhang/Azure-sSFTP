
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type HttpClient struct {
	confsvc *ConfigService
}

type VirusDetectedWebhookPostData struct {
	FileName string			`json:"fileName"`
	MountFilePath string	`json:"mountFilePath"`
	TimeGenerated string	`json:"timeGenerated"`
}

func NewHttpClient(confsvc *ConfigService) (HttpClient) {
	return HttpClient{
		confsvc: confsvc,
	}
}

func (hc HttpClient) callWebhook(virusDetectedFileName string, mountFilePath string) {

	b, jerr := json.Marshal(VirusDetectedWebhookPostData{
		FileName: virusDetectedFileName,
		MountFilePath: mountFilePath,
		TimeGenerated: (time.Now()).Format(time.ANSIC),
	})

	if !logclient.ErrIf(jerr) {

		vfUrl := hc.confsvc.config.getWebHook(VirusFound)
		if isValidUrl(vfUrl) {

			logclient.Infof("Calling webhook %s with params %s", vfUrl, string(b))
			resp, herr := http.Post(vfUrl, "application/json", bytes.NewBuffer(b) )
			
			if !logclient.ErrIf(herr) {
				logclient.Infof("Webhook invokation status %s", resp.Status)
			}

		} else {
			logclient.ErrIf(errors.New("Webhook Url is invalid"))
		}
	}
}