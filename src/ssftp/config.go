

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
	CleanPath = "/mnt/ssftp/clean"
	QuarantinePath = "/mnt/ssftp/quarantine"
	ErrorPath =  "/mnt/ssftp/error"
)

type SSFTPYaml struct {
	SftpPort    int					`json:"sftpPort, yaml:"sftpPort"`
	EnableVirusScan bool			`json:"enableVirusScan, yaml:"enableVirusScan"`
	LogDests []LogDest				`json:"logDests", yaml:"logDests"`
	Users SSFTPYamlUsers			`json:"users", yaml:"users"`
	Webhooks []Webhook				`json:"webhooks", yaml:"webhooks"`
	SFTPClientConnectors []SFTPClientConnector			`json:"sftpClientConnectors", yaml:"sftpClientConnectors"`
}

type  SSFTPYamlUsers struct {
	StagingDirUsers []user.User		`json:"stagingDir", yaml:"stagingDir"`
	CleanDirUsers []user.User		`json:"cleanDir", yaml:"cleanDir"`
}

type ConfigService struct {
	config *Config
	mux    *sync.RWMutex
}

type SFTPClientConnector struct {
	Name string							`json:"name, yaml:"name"`
	ClientType string					`json:"name, yaml:"name"`
    Host string							`json:"host, yaml:"host"`
    Port int 							`json:"port, yaml:"port"`
	Username string						`json:"username, yaml:"username"`
    Password string						`json:"password, yaml:"password"`
    PrivatekeyPath string				`json:"privatekeyPath, yaml:"privatekeyPath"`
    LocalStagingDirectory string		`json:"localStagingDirectory, yaml:"localStagingDirectory"`
    RemoteDirectory string				`json:"remoteDirectory, yaml:"remoteDirectory"`
	DeleteRemoteFileAfterDownload bool	`json:"deleteRemoteFileAfterDownload, yaml:"deleteRemoteFileAfterDownload"`
    OverrideExistingFile bool			`json:"overrideExistingFile, yaml:"overrideExistingFile"`
}

type Config struct {
	SftpPort    int					`json:"sftpPort, yaml:"sftpPort"`
	EnableVirusScan bool			`json:"enableVirusScan, yaml:"enableVirusScan"`
	StagingPath string				`yaml:"stagingPath"`
	CleanPath string				`yaml:"cleanPath"`
	QuarantinePath string			`yaml:"quarantinePath"`
	ErrorPath string				`yaml:"errorPath"`
	LogDests []LogDest				`json:"logDests", yaml:"logDests"`
	Users []user.User				`json:"users", yaml:"users"`
	Webhooks []Webhook				`json:"webhooks", yaml:"webhooks"`
	SFTPClientConnectors []SFTPClientConnector			`json:"sftpClientConnectors", yaml:"sftpClientConnectors"`
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

func (c ConfigService) LoadYamlConfig() chan Config {

	loaded := make(chan Config)

		go func() { 
			for {
				yamlConfgPath := c.getYamlConfgPath()

				b, err := ioutil.ReadFile(yamlConfgPath)
				if logclient.ErrIfm("Config - error while reading config file", err) {
					time.Sleep(3 * time.Second)
					continue
				}

				yamlSchema := SSFTPYaml{}
				
				yerr := yaml.Unmarshal(b, &yamlSchema)
				if logclient.ErrIfm("Config - error while loading config changes", yerr) {
					time.Sleep(3 * time.Second)
					continue
				}

				if isWindows() { //local dev only
					c.config.StagingPath = "C:\\ssftp\\staging"
					c.config.CleanPath =  "C:\\ssftp\\clean"
					c.config.QuarantinePath =  "C:\\ssftp\\quarantine"
					c.config.ErrorPath =  "C:\\ssftp\\error"
					
				} else {
					c.config.StagingPath = StagingPath
					c.config.CleanPath = CleanPath
					c.config.QuarantinePath = QuarantinePath
					c.config.ErrorPath = ErrorPath
				}

				c.mux.Lock()

				c.config.SftpPort = yamlSchema.SftpPort
				c.config.Webhooks = yamlSchema.Webhooks
				c.config.LogDests = yamlSchema.LogDests
				c.config.EnableVirusScan = yamlSchema.EnableVirusScan
				c.config.Users = c.mergeStagingCleanDirUsers(yamlSchema)
				c.config.SFTPClientConnectors = yamlSchema.SFTPClientConnectors

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