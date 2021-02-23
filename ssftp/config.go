

package main

import (
	"time"
	"fmt"
	"io/ioutil"
	"log"
	//"os"
	"runtime"
	//"path/filepath"
	"github.com/goccy/go-yaml"
)

const (
	SystemConfigPath = "/mnt/ssftp/system/ssftp.yaml"
	SystemConfigFileName = "ssftp.yaml"
	StagingPath = "/mnt/ssftp/staging"
	CleanPath = "/mnt/ssftp/clean"
	QuarantinePath = "/mnt/ssftp/quarantine"
	ErrorPath =  "/mnt/ssftp/error"
)

type ConfigService struct {
	config *Config
}

type Config struct {
	SftpPort    int					`json:"sftpPort, yaml:"sftpPort"`
	AllDirFollowUserDir bool		`json:"allDirFollowUserDir, yaml:"allDirFollowUserDir"`
	StagingPath string				//`json:"stagingPath, yaml:"stagingPath"`
	CleanPath string				//`json:"cleanPath, yaml:"cleanPath"`
	QuarantinePath string			//`json:"quarantinePath, yaml:"quarantinePath"`
	ErrorPath string				//`json:"errorPath", yaml:"errorPath"`
	LogDests[]LogDest				`json:"logDests", yaml:"logDests"`
	Users []User					`json:"users", yaml:"users"`
	Webhooks []Webhook				`json:"webhooks", yaml:"webhooks"`
}

type User struct {
	Name string			`json:"name", yaml:"name"`
	Password string		`json:"password", yaml:"password"`
	Directory string	`json:"directory", yaml:"directory"`
	Readonly  bool		`json:"readonly", yaml:"readonly"`
}

type Webhook struct {
	Name string			`json:"name", yaml:"name"`
	Url string			`json:"url", yaml:"url"`
}

type LogDest struct {
	Kind string
	Properties Props	`json:"props", yaml:"props"`
}
type Props map[string]string	

const (
	VirusFound string = "virusFound"
)

 func NewConfigService() (ConfigService) {
	 return ConfigService{
		 config: &Config{},
	}
 }

// 	return &Config{}
// 	// conf := Config{
// 	// 	StagingPath: os.Getenv("stagingPath"),
// 	// 	CleanPath: os.Getenv("cleanPath"),
// 	// 	QuarantinePath: os.Getenv("quarantinePath"),
// 	// 	ErrorPath: os.Getenv("errorPath"),
// 	// 	LogPath: os.Getenv("logPath"),
// 	// 	SystemPath: os.Getenv("systemPath"),
// 	// 	// VirusFoundWebhookUrl: os.Getenv("virusFoundWebhookUrl"),
// 	// }

// 	// if conf.StagingPath == "" || conf.CleanPath == "" || conf.QuarantinePath == "" || conf.ErrorPath == "" {
// 	// 	err := errors.New("Environment variables missing for stagingPath, cleanPath, quarantinePath or errorPath")
// 	// 	log.Fatalln(err)
// 	// 	return conf, err
// 	// }

// 	// return conf, nil
// }

func (c ConfigService) LoadYamlConfig() chan bool {

	loaded := make(chan bool)

		go func(){ 
			for {
				yamlConfgPath := c.getYamlConfgPath()

				b, err := ioutil.ReadFile(yamlConfgPath)
				if logclient.ErrIf(err) {
					time.Sleep(3 * time.Second)
					continue
				}
				
				yerr := yaml.Unmarshal(b, c.config)
				if logclient.ErrIf(yerr) {
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

				configJStr := ToJsonString(c.config)
				log.Println(fmt.Sprintf("sSFTP loaded config from /mnt/ssftp/system/ssftp.yaml: %s", configJStr))

				break
			}

			loaded <- true
		}()
	
	return loaded
}

func (c ConfigService) getYamlConfgPath() string {
	if runtime.GOOS != "windows" {
		return SystemConfigPath
	} else {
		return "ssftp.yaml"
	}
}

func (c *Config) isLogDestConfigured(kind string) (bool) {
	for _, v := range c.LogDests {
		if v.Kind == kind {
			return true
		}
	}
	return false
}

func (c *Config) getLogDestProp(kind string, prop string) (string) {
	for _, v := range c.LogDests {
		if v.Kind == kind {
			propVal := v.Properties[prop]
			return propVal
		}
	}
	return ""
}

func (c *Config) getWebHook(kind string) string {

	for _, v := range c.Webhooks {
		if v.Name == kind {
			return v.Url
		}
	}

	return ""
}