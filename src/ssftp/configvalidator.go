package main

import (
	"fmt"
	"net/url"
	"strings"
)

type StringSlice []string
var ValidWebhookNames StringSlice= StringSlice{"virusFound"}

type ConfigValidator struct {
	ok bool
	erroMessages StringSlice
}

func (cv *ConfigValidator) validateAndSetDefault(config *Config) (string, bool) {
	cv.validatePortSetDefault(config)
	cv.validateWebhook(config)
	
	return cv.erroMessages.toNewlineString(), cv.ok
}

func (cv *ConfigValidator) addErrf(errMsgTemplate string, args ...interface{}) {
	cv.erroMessages = append(cv.erroMessages, fmt.Sprintf(errMsgTemplate, args...))
	if cv.ok {
		cv.ok = false
	}
}

func (cv *ConfigValidator) validatePortSetDefault(config *Config) {
	if config.SftpPort == 0 {
		config.SftpPort = 22
	}
}

func (cv *ConfigValidator) validateWebhook(config *Config) {
	if len(config.Webhooks) > 0 {
		for _, v := range config.Webhooks {
			if !cv.isValidWebhookName(v.Name) {
				cv.addErrf("Webhook name %s is invalid. Valid names are %s", v.Name, ValidWebhookNames.toDelimitedString())
			}

			_, err := url.Parse(v.Url)
			if err != nil {
				cv.addErrf("Webhook Url %s is invalid", v.Url)
			}
		}
	}
}

func (cv *ConfigValidator) isValidWebhookName(name string) bool {
	
	for _, w := range ValidWebhookNames {
		if w == name {
			return true
		}
	}

	return false
}

func (s StringSlice) toNewlineString() (string) {
	var notepad string

	for _, v := range s {
		notepad += v
		notepad += "\n"
	}
	return notepad
}

func (s StringSlice) toDelimitedString() (string) {
	return strings.Join(s, ", ")
}
