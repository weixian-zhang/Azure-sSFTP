
package user

import (
	"os"
	"path/filepath"
)

type User struct {
	Name string				`json:"name", yaml:"name"`
	Password string			`json:"password", yaml:"password"`
	JailDirectory string	`json:"directory", yaml:"directory"`
	Readonly  bool			`json:"readonly", yaml:"readonly"`
}


type UserGov struct {
	Users []User
}

//NewUserRepo has nil Users until LoadUsers is called
func NewUserGov(users []User) UserGov {
	
	return UserGov{
		Users: users,
	}
}

func (ug *UserGov) SetUsers(users []User) {
	ug.Users = users
}

func (ug *UserGov) Auth(name string, pass string) (User, bool) {
	for _, v := range ug.Users {
		if v.Name == name && v.Password == pass {
			return v, true
		}
	}
	return User{}, false
}

func (ug *UserGov) CreateUserDir(baseDir string, name string) {

	dirPath := filepath.Join(baseDir, name)

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		os.Mkdir(dirPath, 0777)
	} 
}