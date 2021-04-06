

package main

import (
	"time"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"github.com/goccy/go-yaml"
	"github.com/weixian-zhang/ssftp/user"
)

const (
	SystemConfigPath = "/mnt/ssftp/system/ssftp.yaml"
	SystemConfigFileName = "ssftp.yaml"
	StagingPath = "/mnt/ssftp/staging"
	LocalRemoteUploadArchiveBasePath =  "/mnt/ssftp/uploadarchive"
	CleanPath = "/mnt/ssftp/clean"
	QuarantinePath = "/mnt/ssftp/quarantine"
	ErrorPath =  "/mnt/ssftp/error"
)

const (
	VirusFoundWebook = "virusFound"
	VirusScanCompleteWebook = "virusScanComplete"
)

type SSFTPYaml struct {
	SftpPort    int					`yaml:"sftpPort"`
	EnableVirusScan bool			`yaml:"enableVirusScan"`
	EnableFileScavenging bool		`yaml:"enableFileScavenging"`
	EnableSftpClientDownloader bool `yaml:"enableSftpClientDownloader"`
	EnableSftpClientUploader bool	`yaml:"enableSftpClientUploader"`
	LogDests []LogDest				`yaml:"logDests"`
	Users SSFTPYamlUsers			`yaml:"users"`
	Webhooks []Webhook				`yaml:"webhooks"`
	ClientDownloaders []ClientDownloader	`yaml:"sftpClientDownloaders"`
	ClientUploaders []ClientUploader	`yaml:"sftpClientUploaders"`
}

type  SSFTPYamlUsers struct {
	StagingDirUsers []user.User		`yaml:"stagingDir"`
	CleanDirUsers []user.User		`yaml:"cleanDir"`
}

type ConfigService struct {
	config *Config
	mux    *sync.RWMutex
}

type ClientDownloader struct {
	Name string							`yaml:"name"`
    Host string							`yaml:"host"`
    Port int 							`yaml:"port"`
	Username string						`yaml:"username"`
    Password string						`yaml:"password"`
    PrivatekeyPath string				`yaml:"privateKeyPath"`
	PrivatekeyPassphrase string			`yaml:"privatekeyPassphrase"`
    LocalStagingDirectory string		`yaml:"localStagingDirectory"`
    RemoteDirectory string				`yaml:"remoteDirectory"`
	DeleteRemoteFileAfterDownload bool	`yaml:"deleteRemoteFileAfterDownload"`
    OverrideExistingFile bool			`yaml:"overrideExistingFile"`
}

type ClientUploader struct {
	Name string							`yaml:"name"`
    Host string							`yaml:"host"`
    Port int 							`yaml:"port"`
	Username string						`yaml:"username"`
    Password string						`yaml:"password"`
    PrivatekeyPath string				`yaml:"privateKeyPath"`
	PrivatekeyPassphrase string			`yaml:"privatekeyPassphrase"`
    LocalDirectoryToUpload string		`yaml:"localDirectoryToUpload"`
    RemoteDirectory string				`yaml:"remoteDirectory"`
    OverrideRemoteExistingFile bool		`yaml:"overrideRemoteExistingFile"`
}

type Config struct {
	SftpPort    int						`yaml:"sftpPort"`
	EnableVirusScan bool				`yaml:"enableVirusScan"`
	EnableFileScavenging bool			`yaml:"enableFileScavenging"`
	EnableSftpClientDownloader bool 	`yaml:"enableSftpClientDownloader"`
	EnableSftpClientUploader bool		`yaml:"enableSftpClientUploader"`
	StagingPath string					`yaml:"stagingPath"`
	LocalRemoteUploadArchiveBasePath string `yaml:"localRemoteUploadArchiveBasePath"`
	CleanPath string					`yaml:"cleanPath"`
	QuarantinePath string				`yaml:"quarantinePath"`
	ErrorPath string					`yaml:"errorPath"`
	LogDests []LogDest					`yaml:"logDests"`
	Users []user.User					`yaml:"users"`
	Webhooks []Webhook					`yaml:"webhooks"`
	ClientDownloaders []ClientDownloader `yaml:"sftpClientDownloaders"`
	ClientUploaders []ClientUploader	`yaml:"sftpClientUploaders"`
}

type Webhook struct {
	Name string			`json:"name", yaml:"name"`
	Url string			`json:"url", yaml:"url"`
}

type LogDest struct {
	Kind string			`json:"kind", yaml:"kind"`
	Properties Props	`json:"props", yaml:"props"`
}
type Props map[string]string	

const (
	VirusFound string = "virusFound"
)

 func NewConfigService() (ConfigService) {
	 return ConfigService{
		 config: &Config{},
		 mux: &sync.RWMutex{},
	}
 }

func (c *ConfigService) LoadYamlConfig() chan Config {

	loaded := make(chan Config)

		go func() { 
			for {
				yamlConfgPath := c.getYamlConfgPath()

				b, err := ioutil.ReadFile(yamlConfgPath)
				if logclient.ErrIfm("Config - error reading config file", err) {
					time.Sleep(3 * time.Second)
					continue
				}

				yamlSchema := SSFTPYaml{}
				
				yerr := yaml.Unmarshal(b, &yamlSchema)
				if logclient.ErrIfm("Config - error unmarshaling config changes", yerr) {
					time.Sleep(3 * time.Second)
					continue
				}

				if os.Getenv("env") == "dev" { //local dev only
					c.config.StagingPath = "/mnt/c/ssftp/staging"
					c.config.LocalRemoteUploadArchiveBasePath = "/mnt/c/ssftp/clean/remoteupload-archive"
					c.config.CleanPath =  "/mnt/c/ssftp/clean"
					c.config.QuarantinePath =  "/mnt/c/ssftp/quarantine"
					c.config.ErrorPath =  "/mnt/c/ssftp/error"
					
				} else {
					c.config.StagingPath = StagingPath
					c.config.LocalRemoteUploadArchiveBasePath = LocalRemoteUploadArchiveBasePath
					c.config.CleanPath = CleanPath
					c.config.QuarantinePath = QuarantinePath
					c.config.ErrorPath = ErrorPath
				}

				c.mux.Lock()

				c.config.SftpPort = yamlSchema.SftpPort
				c.config.Webhooks = yamlSchema.Webhooks
				c.config.LogDests = yamlSchema.LogDests
				c.config.EnableVirusScan = yamlSchema.EnableVirusScan
				c.config.EnableFileScavenging = yamlSchema.EnableFileScavenging
				c.config.EnableSftpClientDownloader = yamlSchema.EnableSftpClientDownloader
				c.config.EnableSftpClientUploader = yamlSchema.EnableSftpClientUploader
				c.config.EnableVirusScan = yamlSchema.EnableVirusScan
				c.config.Users = c.mergeStagingCleanDirUsers(yamlSchema)
				c.config.ClientDownloaders = yamlSchema.ClientDownloaders
				c.config.ClientUploaders = yamlSchema.ClientUploaders

				c.mux.Unlock()



				y, yerr := yaml.Marshal(c.config)
				logclient.ErrIfm("Config - error while marshaling to Yaml string for display", yerr)

				configJStr := string(y)
				log.Println(fmt.Sprintf("Config - loaded config from %s: %s", yamlConfgPath, configJStr))

				loaded <- *c.config

				break
			}
			
		}()
	
	return loaded
}

func (c *ConfigService) mergeStagingCleanDirUsers(ssftpyaml SSFTPYaml) []user.User {

	users := make([]user.User, 0)

	for _, v := range ssftpyaml.Users.StagingDirUsers {
		users = append(users, v)
	}

	for _, v := range ssftpyaml.Users.CleanDirUsers {
		v.IsCleanDirUser = true
		users = append(users, v)
	}

	return users
}

func (c *ConfigService) getYamlConfgPath() string {
	if os.Getenv("env") != "" && os.Getenv("env") == "dev" {
		return "/mnt/c/weixian/projects/Azure-Scanned-File-Transfer/src/ssftp/ssftp.yaml"
	} else {
		return SystemConfigPath
	}
}

// func (c *ConfigService) isLogDestConfigured(kind string) (bool) {
// 	for _, v := range c.config.LogDests {
// 		if v.Kind == kind {
// 			return true
// 		}
// 	}
// 	return false
// }

func (c *ConfigService) GetLogDestProp(kind string, prop string) (string) {
	for _, v := range c.config.LogDests {
		if v.Kind == kind {
			propVal := v.Properties[prop]
			return propVal
		}
	}
	return ""
}

func (c *ConfigService) getWebHook(kind string) string {

	for _, v := range c.config.Webhooks {
		if v.Name == kind {
			return v.Url
		}
	}

	return ""
}