
package main

import (
	"os"
	"path/filepath"
)


type UserGov struct {
	users []User
	config Config
}

//NewUserRepo has nil Users until LoadUsers is called
func NewUserGov(conf Config) UserGov {
	
	return UserGov{
		config: conf,
		users: nil,
	}
}

func (ug UserGov) Auth(name string, pass string) (User, bool) {
	for _, v := range ug.config.Users {
		if v.Name == name && v.Password == pass {
			return v, true
		}
	}
	return User{}, false
}

func (ug UserGov) CreateUserDir(name string) {

	dirPath := filepath.Join(ug.config.StagingPath, name)

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		os.Mkdir(dirPath, 0777)
		logclient.Infof("Created user directory at %s", dirPath)
	} else {
		logclient.Infof("Skip creation user directory exist at %s", dirPath)
	}
}

// func (ug UserGov) createSftpSvcRoutes() ([]Route) {
// 	routes := make([]Route,0)

// 	for _, v := range ug.config.Users {
// 		routes = append(routes, Route{
// 			Username: v.Name,
// 			Password: v.Password,
// 			Endpoint: filepath.Join(ug.config.StagingPath, v.Directory),
// 		})
// 	}

// 	return routes
// }

// User is created by executing shell command useradd
// func (ug UserGov) AddNewUser(dir string, name string, pass string) (error) {

// 	if ug.isUserExist(name) {
// 		logclient.Infof("User %s exist", name)
// 		return nil
// 	}

// 	argUser := []string{"-H", "-h", dir, "-G", "admin", "-D", "-s", "/bin/sh", name}
// 	argPass := []string{"-c", fmt.Sprintf("echo %s:%s | chpasswd", name, pass)}

// 	userCmd := exec.Command("adduser", argUser...)
// 	passCmd := exec.Command("/bin/sh", argPass...)

// 	if _, err := userCmd.Output(); err != nil {
// 		logclient.ErrIfm(fmt.Sprintf("Error adding user %s", name), err)
// 		return err
// 	} else {
// 		if _, err := passCmd.Output(); err != nil {
// 			logclient.ErrIfm(fmt.Sprintf("Error when setting password for user %s", name), err)
// 			return err
// 		} else {
// 			logclient.Infof("Password successfully set for user %s", name)
			
// 		}
// 		logclient.Infof("User %s successfully created ", name)
// 		return nil
// 	}
// }

// func (ug UserGov) isUserExist(name string) bool {

// 	_, err := user.Lookup(name)

// 	if _, ok := err.(user.UnknownUserError); ok {
// 		logclient.ErrIfm(fmt.Sprintf("User %s does not exist", name), err)
// 		return false
// 	} else {
// 		return true
// 	}
// }

// func (ug UserGov) createAdminGroup() {

// 	if ug.isAdminGroupExist() {
// 		return
// 	}
	
// 	logclient.Info("Creating admin user group")

//     arg := []string {"-g", "499", "-S", "admin"}

// 	grpCmd := exec.Command("addgroup", arg...)

// 	r, err:= grpCmd.Output()

// 	if isErr(err) {
// 		logclient.ErrIfm("Error creating user group admin id=499", err)
// 	} else {
// 		logclient.Infof("Admin user group created %s", string(r))
// 	}
// }

// func (ug UserGov) isAdminGroupExist() (bool) {
// 	_, err := user.LookupGroupId("499")

// 	if  _, ok := err.(user.UnknownGroupIdError); ok {
// 		return false
// 	} else {
// 		return true
// 	}
// }