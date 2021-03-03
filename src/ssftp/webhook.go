
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

type HttpClient struct {
	confsvc *ConfigService
}

type VirusDetectedWebhookData struct {
	Username string			`json:"username"`
	FileName string			`json:"fileName"`
	ScanMessage  string		`json:"scanMessage"`
	FilePath string			`json:"mountFilePath"`
	TimeGenerated string	`json:"timeGenerated"`
}

func NewHttpClient(confsvc *ConfigService) (HttpClient) {
	return HttpClient{
		confsvc: confsvc,
	}
}

func (hc HttpClient) callVirusFoundWebhook(data VirusDetectedWebhookData) {

	vfUrl := hc.confsvc.getWebHook(VirusFound)
	if isValidUrl(vfUrl) {

		b, jerr := json.Marshal(data)
		logclient.ErrIf(jerr)

		logclient.Infof("Calling webhook %s with params %s", vfUrl, string(b))
		resp, herr := http.Post(vfUrl, "application/json", bytes.NewBuffer(b) )
		
		if !logclient.ErrIf(herr) {
			logclient.Infof("Webhook invokation status %s", resp.Status)
		}

	} else {
		logclient.ErrIf(errors.New("Webhook Url is invalid"))
	}
}