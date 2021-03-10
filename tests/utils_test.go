package cos_test

import (
	"os"
	"time"

	. "github.com/onsi/gomega"
	ssh "golang.org/x/crypto/ssh"
)

func eventuallyConnects(t int) {
	Eventually(func() error {
		_, _, err := connectToHost()
		return err
	}, time.Duration(time.Duration(t)*time.Second), time.Duration(5*time.Second)).ShouldNot(HaveOccurred())
}

func sshCommand(cmd string) (string, error) {

	client, session, err := connectToHost()
	if err != nil {
		return "", err
	}
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(out), err
	}
	client.Close()

	return string(out), err
}

func connectToHost() (*ssh.Client, *ssh.Session, error) {
	user := os.Getenv("COS_USER")
	if user == "" {
		user = "root"
	}
	pass := os.Getenv("COS_PASS")
	if pass == "" {
		pass = "cos"
	}

	host := os.Getenv("COS_HOST")
	if host == "" {
		host = "127.0.0.1:2222"
	}

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(pass)},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
}
