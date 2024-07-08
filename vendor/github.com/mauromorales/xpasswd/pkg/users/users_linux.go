package users

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LinuxUser matches the fields in /etc/passwd. See man 5 passwd for more information
type LinuxUser struct {
	login                  string
	password               string
	uid                    string
	gid                    string
	userNameOrComment      string
	userHomeDir            string
	usercommandInterpreter string
}

func NewUserList() UserList {
	return &LinuxUserList{path: "/etc/passwd"}
}

// LinuxUserList is a list of Linux users
type LinuxUserList struct {
	CommonUserList
	path string
}

func (u LinuxUser) UID() (int, error) {
	return strconv.Atoi(u.uid)
}

func (u LinuxUser) GID() (int, error) {
	return strconv.Atoi(u.gid)
}

func (u LinuxUser) Username() string {
	return u.login
}

func (u LinuxUser) Password() string {
	return u.password
}

func (u LinuxUser) HomeDir() string {
	return u.userHomeDir
}

func (u LinuxUser) Shell() string {
	return u.usercommandInterpreter
}

func (u LinuxUser) RealName() string {
	return u.userNameOrComment
}

func (l *LinuxUserList) SetPath(path string) {
	l.path = path
}

func (l *LinuxUserList) Load() error {
	_, err := l.GetAll()
	return err
}

// GetAll returns all users in the list
func (l *LinuxUserList) GetAll() ([]User, error) {
	users := make([]User, 0)

	file, err := os.Open(l.path)
	if err != nil {
		return users, err
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Read each line
	for scanner.Scan() {
		line := scanner.Text()

		user, lineErr := parseRecord(line)
		if lineErr == nil {
			users = append(users, user)
		}

		uid, lineErr := user.UID()
		if lineErr != nil {
			err = lineErr
		}

		if lineErr == nil && uid > l.lastUID {
			l.lastUID = uid
		}
	}

	// Check if there were errors during scanning
	if err := scanner.Err(); err != nil {
		return users, fmt.Errorf("error reading the file: %w", err)
	}

	l.users = users

	return users, err
}

func parseRecord(record string) (LinuxUser, error) {
	user := LinuxUser{}
	fields := strings.Split(record, ":")
	// Check if the line is correctly formatted with 7 fields
	if len(fields) != 7 {
		return user, fmt.Errorf("unexpected format: %s", record)
	}

	user.login = fields[0]
	user.password = fields[1]
	user.uid = fields[2]
	user.gid = fields[3]
	user.userNameOrComment = fields[4]
	user.userHomeDir = fields[5]
	user.usercommandInterpreter = fields[6]

	return user, nil
}
