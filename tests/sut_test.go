package cos_test

import (
	"net"
	"os"
	"time"

	. "github.com/onsi/gomega"
	ssh "golang.org/x/crypto/ssh"
)

type SUT struct {
	Host     string
	Username string
	Password string
}

func NewSUT(Host, Username, Password string) *SUT {
	user := os.Getenv("COS_USER")
	if Username == "" {
		user = "root"
	}
	pass := os.Getenv("COS_PASS")
	if Password == "" {
		pass = "cos"
	}

	host := os.Getenv("COS_HOST")
	if Host == "" {
		host = "127.0.0.1:2222"
	}
	return &SUT{
		Host:     host,
		Username: user,
		Password: pass,
	}
}

func (s *SUT) EventuallyConnects(t ...int) {
	dur := 360
	if len(t) > 0 {
		dur = t[0]
	}
	Eventually(func() string {
		out, _ := s.Command("echo ping", true)
		return out
	}, time.Duration(time.Duration(dur)*time.Second), time.Duration(5*time.Second)).Should(Equal("ping\n"))
}

func (s *SUT) Command(cmd string, timeout bool) (string, error) {
	client, err := s.connectToHost(timeout)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}

	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(out), err
	}

	return string(out), err
}

func (s *SUT) Reboot() {
	s.Command("reboot", true)
	time.Sleep(120 * time.Second)
}

func (s *SUT) connectToHost(timeout bool) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:    s.Username,
		Auth:    []ssh.AuthMethod{ssh.Password(s.Password)},
		Timeout: 30 * time.Second, // max time to establish connection
	}

	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := DialWithDeadline("tcp", s.Host, sshConfig, timeout)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func DialWithDeadline(network string, addr string, config *ssh.ClientConfig, timeout bool) (*ssh.Client, error) {
	conn, err := net.DialTimeout(network, addr, config.Timeout)
	if err != nil {
		return nil, err
	}
	if config.Timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(config.Timeout))
		conn.SetWriteDeadline(time.Now().Add(config.Timeout))
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	if !timeout {
		conn.SetReadDeadline(time.Time{})
		conn.SetWriteDeadline(time.Time{})
	}

	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for range t.C {
			_, _, err := c.SendRequest("keepalive@golang.org", true, nil)
			if err != nil {
				return
			}
		}
	}()
	return ssh.NewClient(c, chans, reqs), nil
}
