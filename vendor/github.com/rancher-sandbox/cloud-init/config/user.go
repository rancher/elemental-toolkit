// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

type User struct {
	Name                 string   `yaml:"name,omitempty"`
	PasswordHash         string   `yaml:"passwd,omitempty"`
	SSHAuthorizedKeys    []string `yaml:"ssh_authorized_keys,omitempty"`
	SSHImportGithubUser  string   `yaml:"coreos_ssh_import_github,omitempty"       deprecated:"trying to fetch from a remote endpoint introduces too many intermittent errors"`
	SSHImportGithubUsers []string `yaml:"coreos_ssh_import_github_users,omitempty" deprecated:"trying to fetch from a remote endpoint introduces too many intermittent errors"`
	SSHImportURL         string   `yaml:"coreos_ssh_import_url,omitempty"          deprecated:"trying to fetch from a remote endpoint introduces too many intermittent errors"`
	GECOS                string   `yaml:"gecos,omitempty"`
	Homedir              string   `yaml:"homedir,omitempty"`
	NoCreateHome         bool     `yaml:"no_create_home,omitempty"`
	PrimaryGroup         string   `yaml:"primary_group,omitempty"`
	Groups               []string `yaml:"groups,omitempty"`
	NoUserGroup          bool     `yaml:"no_user_group,omitempty"`
	System               bool     `yaml:"system,omitempty"`
	NoLogInit            bool     `yaml:"no_log_init,omitempty"`
	Shell                string   `yaml:"shell,omitempty"`
	LockPasswd           bool     `yaml:"lock_passwd"`
	UID                  string   `yaml:"uid"`
}
