
package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"github.com/weixian-zhang/ssftp/logc"
	"net/url"
)

type HttpClient struct {
	logclient *logc.LogClient
}

type VirusDetectedWebhookData struct {
	Username string			`json:"username"`
	ScanMessage  string		`json:"scanMessage"`
	FilePath string			`json:"filePath"`
	TimeGenerated string	`json:"timeGenerated"`
}

type virusScanCompletedWebhookData struct {
	Username string			`json:"username"`
	LocalFilePath string	`json:"filePath"`
	TimeGenerated string	`json:"timeGenerated"`
} 

func NewHttpClient(logclient *logc.LogClient) (HttpClient) {
	return HttpClient{
		logclient: logclient,
	}
}

func (hc *HttpClient) CallVirusFoundWebhook(url string, data VirusDetectedWebhookData) {

	if hc.isValidUrl(url) {

		b, jerr := json.Marshal(data)
		hc.logclient.ErrIf(jerr)

		hc.logclient.Infof("Calling webhook %s with params %s", url, string(b))
		resp, herr := http.Post(url, "application/json", bytes.NewBuffer(b) )
		
		if !hc.logclient.ErrIf(herr) {
			hc.logclient.Infof("Webhook invokation status %s", resp.Status)
		}

	} else {
		hc.logclient.ErrIf(errors.New("Webhook Url is invalid"))
	}
}



func (hc *HttpClient) isValidUrl(urlp string) (bool) {
	_, err := url.Parse(urlp)

	if hc.logclient.ErrIf(err) {
		return false
	} else {
		return true
	}
}