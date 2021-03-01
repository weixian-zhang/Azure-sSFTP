
package user

import (
	"os"
	"path/filepath"
	"golang.org/x/crypto/ssh"
)

type User struct {
	Name string				`json:"name", yaml:"name"`
	JailDirectory string	`json:"directory", yaml:"directory"`
	Readonly  bool			`json:"readonly", yaml:"readonly"`
	Auth Auth				`json:"auth", yaml:"auth"`
	IsInternalUser bool		`json:"isInternalUser"`
}

type Auth struct {
	Password string  		`json:"password", yaml:"password"`
	PublicKey string  		`json:"publicKey", yaml:"publicKey"`
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

func (ug *UserGov) AuthPass(name string, pass string) (User, bool) {
	for _, v := range ug.Users {
		if v.Name == name && v.Auth.Password == pass {
			return v, true
		}
	}
	return User{}, false
}

func (ug *UserGov) AuthPublicKey(name string, pubKey ssh.PublicKey) (User, bool, error) {
	for _, v := range ug.Users {

		if v.Name == name {

			usrAuthPKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(v.Auth.PublicKey))

			if err != nil {
				return v, false, err
			}

			usrPubKeyStr := string(usrAuthPKey.Marshal())
			passedInPubKey := string(pubKey.Marshal())

			if usrPubKeyStr  == passedInPubKey {
				return v, true, nil 
			} 

		}

		
	}
	return User{}, false, nil
}

func (ug *UserGov) CreateUserDir(baseDir string, name string) {

	dirPath := filepath.Join(baseDir, name)

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		os.Mkdir(dirPath, 0777)
	} 
}