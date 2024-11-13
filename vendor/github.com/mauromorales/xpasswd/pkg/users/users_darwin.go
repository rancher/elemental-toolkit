package users

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DarwinUser matches the fields in ds
type DarwinUser struct {
	recordName       string
	password         string
	uniqueID         string
	primaryGroupID   string
	realName         string
	nFSHomeDirectory string
	userShell        string
}

func (u DarwinUser) UID() (int, error) {
	return strconv.Atoi(u.uniqueID)
}

func (u DarwinUser) GID() (int, error) {
	return strconv.Atoi(u.primaryGroupID)
}

func (u DarwinUser) Username() string {
	return u.recordName
}

func (u DarwinUser) Password() string {
	return u.password
}

func (u DarwinUser) HomeDir() string {
	return u.nFSHomeDirectory
}

func (u DarwinUser) Shell() string {
	return u.userShell
}

func (u DarwinUser) RealName() string {
	return u.realName
}

func NewUserList() UserList {
	return &DarwinUserList{}
}

// DarwinUserList is a list of Linux users
type DarwinUserList struct {
	CommonUserList
}

func (l *DarwinUserList) SetPath(path string) {
}

func (l DarwinUserList) Load() error {
	_, err := l.GetAll()
	return err
}

// GetAll returns a list of users on a Darwin system
func (l DarwinUserList) GetAll() ([]User, error) {
	users := make([]User, 0)

	output, err := execDSCL("-readall", "/Users", "UniqueID", "PrimaryGroupID", "RealName", "UserShell", "NFSHomeDirectory", "RecordName", "Password")
	if err != nil {
		return users, fmt.Errorf("failed to execute command: %w", err)
	}

	records := strings.Split(strings.TrimSpace(output), "\n-\n")

	for _, record := range records {
		user := parseRecord(record)
		users = append(users, user)
		uid, err := user.UID()
		if err != nil {
			return users, fmt.Errorf("failed to convert UID to int: %w", err)
		}
		if uid > l.lastUID {
			l.lastUID = uid
		}
	}

	l.users = users

	return users, nil
}

func parseRecord(record string) DarwinUser {
	user := DarwinUser{}
	lines := strings.Split(strings.TrimSpace(record), "\n")
	preK := ""
	for _, line := range lines {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")

		key := k
		val := strings.TrimSpace(v)

		if ok {
			preK = k
		} else if !ok && preK != "" {
			key = preK
			val = k
			preK = ""
		}

		switch key {
		case "RecordName":
			user.recordName = val
		case "Password":
			user.password = val
		case "UniqueID":
			user.uniqueID = val
		case "PrimaryGroupID":
			user.primaryGroupID = val
		case "RealName":
			user.realName = val
		case "NFSHomeDirectory":
			user.nFSHomeDirectory = val
		case "UserShell":
			user.userShell = val
		}
	}
	return user
}

func execDSCL(arg ...string) (string, error) {
	args := append([]string{"."}, arg...)
	cmd := exec.Command("dscl", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}
