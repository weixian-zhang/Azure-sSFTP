package main

import (
	"time"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	
	"fmt"
	"log"
	"os"
	"io"
	"path/filepath"
	"sync"
)

var files map[int]string

const host string = "40.65.169.72"
const port int = 2002
const remotefile string = "/mnt/ssftp/staging/staginguploaderuser2/20GB.zip"
const localfile string = "/mnt/c/ssftp"


type User struct {
	Name string
	Password string
	RemoteDir string
	LocaleDir string
}
var users []User

var wg sync.WaitGroup

func main() {

	user1 := User{
		Name: "staginguploaderuser1",
		Password: "pass",
		RemoteDir: "/mnt/ssftp/staging/staginguploaderuser1",
		LocaleDir: localfile,
	}
	user2 := User{
		Name: "staginguploaderuser2",
		Password: "tiger",
		RemoteDir: "/mnt/ssftp/staging/staginguploaderuser2",
	LocaleDir: localfile,
	}
	user3 := User{
		Name: "staginguploaderuser3",
		Password: "tooth",
		RemoteDir: "/mnt/ssftp/staging/staginguploaderuser3",
	LocaleDir: localfile,
	}
	user4 := User{
		Name: "staginguploaderuser4",
		Password: "111",
		RemoteDir: "/mnt/ssftp/staging/staginguploaderuser4",
	LocaleDir: localfile,
	}
	user5 := User{
		Name: "staginguploaderuser5",
		Password: "55555",
		RemoteDir: "/mnt/ssftp/staging/staginguploaderuser5",
	LocaleDir: localfile,
	}
	user6 := User{
		Name: "staginguploaderuser6",
		Password: "666666",
		RemoteDir: "/mnt/ssftp/staging/staginguploaderuser6",
	LocaleDir: localfile,
	}
	user7 := User{
		Name: "staginguploaderuser7",
		Password: "7777777",
		RemoteDir: "/mnt/ssftp/staging/staginguploaderuser7",
	LocaleDir: localfile,
	}
	// user8 := User{
	// 	Name: "staginguploaderuser8",
	// 	Password: "88888888",
	// 	RemoteDir: "/mnt/ssftp/staging/staginguploaderuser8",
	// LocaleDir: localfile,
	// }
	// user9 := User{
	// 	Name: "staginguploaderuser9",
	// 	Password: "999999999",
	// 	RemoteDir: "/mnt/ssftp/staging/staginguploaderuser9",
	// LocaleDir: localfile,
	// }
	// user10 := User{
	// 	Name: "staginguploaderuser10",
	// 	Password: "1000000000",
	// 	RemoteDir: "/mnt/ssftp/staging/staginguploaderuser10",
	// LocaleDir: localfile,
	// }

	users = make([]User, 0)
	users = append(users, user1)
	users = append(users, user2)
	users = append(users, user3)
	users = append(users, user4)
	users = append(users, user5)
	users = append(users, user6)
	users = append(users, user7)
	// users = append(users, user8)
	// users = append(users, user9)
	// users = append(users, user10)

	files = make(map[int]string)
	files[0] = "1GB.zip"
	files[1] = "10GB.zip"
	files[2] = "20GB.zip"

	for _, v := range users {
		//min := 1
		//max := 2
		//num := rand.Intn(max - min) + min

		fileName := filepath.Join(v.LocaleDir, files[2])
		rmtFile:= filepath.Join(v.RemoteDir,  files[2])
		//locFile := v.LocaleDir

		c := NewClient(v.Name, v.Password)

		go upload(c, fileName, rmtFile)

		wg.Add(1)
	}

	wg.Wait()
}

func NewClient(username string, pass string) *sftp.Client {
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		Timeout:         10 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	config.Ciphers = append(config.Config.Ciphers, "aes128-gcm@openssh.com")

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := ssh.Dial("tcp", addr, config)

	if err != nil {
		log.Panicf("Error while connect to SFTP server, Error: %s", err.Error())
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		log.Panicf("Error while creating new sftp client, Error: %s", err.Error())
	}

	time.Sleep(3 * time.Second)

	return client
}

func upload(client *sftp.Client, localFile, remoteFile string) (err error) {
	srcFile, err := os.Open(localFile)
	if err != nil {
		log.Panicf("Error while opening file to upload, Error: %s", err.Error())
		return
	}
	defer srcFile.Close()

	dstFile, err := client.Create(remoteFile)
	if err != nil {
		log.Panicf("Error while creating remote file, Error: %s", err.Error())
		return
	}
	defer dstFile.Close()

	b, err := io.Copy(dstFile, srcFile)
	log.Printf("File upload byes %d", b)

	wg.Done()

	if err != nil {
		log.Panicf("Error while uploading file, Error: %s", err.Error())
		return
	}
	
	return
}