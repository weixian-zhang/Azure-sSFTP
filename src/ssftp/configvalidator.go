package main

import (
	"fmt"
	"net/url"
	"strings"
)

type StringSlice []string
var ValidWebhookNames StringSlice= StringSlice{"virusFound"}

type ConfigValidator struct {
	config *Config
	ok bool
	erroMessages StringSlice
}

func NewConfigValidator(config *Config) (ConfigValidator) {
	return ConfigValidator{
		config: config,
		ok: true,
		erroMessages: make(StringSlice, 0),
	}
}

func (cv *ConfigValidator) Validate() (string, bool) {
	
	cv.validatePortSetDefault()
	cv.validateWebhook()
	cv.validateDownloaders()
	cv.validateUploaders()
	cv.validateUsersCleanDir()
	cv.validateUsersStagingDir()
	
	return cv.erroMessages.toNewlineString(), cv.ok
}

func (cv *ConfigValidator) addErrf(errMsgTemplate string, args ...interface{}) {
	cv.erroMessages = append(cv.erroMessages, fmt.Sprintf(errMsgTemplate, args...))
	if cv.ok {
		cv.ok = false
	}
}

func (cv *ConfigValidator) validatePortSetDefault() {
	if cv.config.SftpPort == 0 {
		cv.config.SftpPort = 2002
	}
}

func (cv *ConfigValidator) validateWebhook() {
	if len(cv.config.Webhooks) > 0 {
		for _, v := range cv.config.Webhooks {
			if !cv.isValidWebhookName(v.Name) {
				cv.addErrf("ConfigValidator - Webhook name %s is invalid. Valid names are %s", v.Name, ValidWebhookNames.toDelimitedString())
			}

			_, err := url.Parse(v.Url)
			if err != nil {
				cv.addErrf("ConfigValidator - Webhook Url %s is invalid", v.Url)
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

func (cv *ConfigValidator) validateDownloaders() {

	var localStgDirs StringSlice
	var downloaderNames StringSlice

	for _, v := range cv.config.ClientDownloaders {
		localStgDirs = append(localStgDirs, v.LocalStagingDirectory)

		//validate names
		if len(v.Name) == 0 {
			cv.addErrf("ConfigValidator - Downloader name cannot be empty", v.Name)
		}

		downloaderNames = append(downloaderNames, v.Name)
	}

	s, hasDupNames := cv.hasDupElements(downloaderNames)
	if hasDupNames {
		cv.addErrf("ConfigValidator - Downloader Name are unique identifier and must be unique, %s", s.toNewlineString())
	}

	s, hasDup := cv.hasDupElements(localStgDirs)
	if hasDup {
		cv.addErrf("ConfigValidator - Downloader LocalStagingDirectory %s is duplicated. This will cause Downloaders to override downloaded files in same LocalStagingDirectory", s.toNewlineString())
	}

	for _, v := range cv.config.ClientDownloaders {
		
		if v.Port == 0 {
			cv.addErrf("ConfigValidator - Downloader %s port cannot be empty")
		}

		if len(v.Host) == 0 {
			cv.addErrf("ConfigValidator - Downloader %s host cannot be empty", v.Name)
		}

		if len(v.LocalStagingDirectory) == 0 {
			cv.addErrf("ConfigValidator - Downloader %s LocalStagingDirectory cannot be empty", v.Name)
		}
	}
}

func (cv *ConfigValidator) validateUploaders() {

	var uploaderNames StringSlice

	for _, v := range cv.config.ClientUploaders {

		//validate names
		if len(v.Name) == 0 {
			cv.addErrf("ConfigValidator - Uploader name cannot be empty")
		}

		uploaderNames = append(uploaderNames, v.Name)
	}

	s, hasDup := cv.hasDupElements(uploaderNames)
	if hasDup {
		cv.addErrf("ConfigValidator - Uploader Name are unique identifier and must be unique, %s", s.toNewlineString())
	}

	for _, v := range cv.config.ClientUploaders {
		
		if v.Port == 0 {
			cv.addErrf("ConfigValidator - Uploader %s port cannot be empty", v.Name)
		}

		if len(v.Host) == 0 {
			cv.addErrf("ConfigValidator - Uploader %s host cannot be empty", v.Name)
		}
	}
}

func (cv *ConfigValidator) hasDupElements(s StringSlice) (StringSlice, bool) {
	uniqueLocalDirChecker := make(map[string]bool, len(s))
	dupElems := make(StringSlice, len(uniqueLocalDirChecker))
	for _, v := range s {
		if !uniqueLocalDirChecker[v] {
			uniqueLocalDirChecker[v] = true
		} else {
			dupElems = append(dupElems, v)
		}
	}
	
	if len(dupElems) == 0 {
		return dupElems, false
	}

	return dupElems, true	
}

func (cv *ConfigValidator) validateUsersCleanDir() {

	for _, v := range cv.config.Users {
		if !v.IsCleanDirUser {
			continue
		}

		if len(v.Auth.Username) == 0 {
			cv.addErrf("ConfigValidator - Users.cleanDir Username cannot be empty")
		}

		if len(v.Auth.Password) == 0 {
			cv.addErrf("ConfigValidator - Users.cleanDir Password cannot be empty even though Public Key authn is used")
		}

		if len(v.JailDirectory) == 0 {
			cv.addErrf("ConfigValidator - Users.cleanDir directory cannot be empty, either (*) to pin to root directory or a given directory name")
		}
	}
}

func (cv *ConfigValidator) validateUsersStagingDir() {
	for _, v := range cv.config.Users {
		if v.IsCleanDirUser {
			continue
		}

		if len(v.Auth.Username) == 0 {
			cv.addErrf("ConfigValidator - Users.stagingDir Username cannot be empty")
		}

		if len(v.Auth.Password) == 0 {
			cv.addErrf("ConfigValidator - Users.stagingDir - Password cannot be empty even though Public Key authn is used")
		}

		if v.JailDirectory == "*" {
			cv.addErrf("ConfigValidator - Users.stagingDir - Root(*) is not permitted in Staging directory, only Clean directory can have Root(*)")
		}

		if len(v.JailDirectory) == 0 {
			cv.addErrf("ConfigValidator - Users.stagingDir - directory cannot be empty")
		}
	}
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
