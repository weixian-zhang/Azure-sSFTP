package sftpclient

func NewClient(username string, pass string, privatekeyPath string) *sftp.Client {
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