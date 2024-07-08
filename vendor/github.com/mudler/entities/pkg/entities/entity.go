/*
Copyright © 2020 Ettore Di Giacinto <mudler@mocaccino.org>
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package entities

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ENTITY_ENV_DEF_GROUPS        = "ENTITY_DEFAULT_GROUPS"
	ENTITY_ENV_DEF_PASSWD        = "ENTITY_DEFAULT_PASSWD"
	ENTITY_ENV_DEF_SHADOW        = "ENTITY_DEFAULT_SHADOW"
	ENTITY_ENV_DEF_GSHADOW       = "ENTITY_DEFAULT_GSHADOW"
	ENTITY_ENV_DEF_DYNAMIC_RANGE = "ENTITY_DYNAMIC_RANGE"
	ENTITY_ENV_DEF_DELAY         = "ENTITY_DEFAULT_DELAY"
	ENTITY_ENV_DEF_INTERVAL      = "ENTITY_DEFAULT_INTERVAL"

	// https://systemd.io/UIDS-GIDS/#summary
	// https://systemd.io/UIDS-GIDS/#special-distribution-uid-ranges
	HumanIDMin = 1000
	HumanIDMax = 60000
)

// Entity represent something that needs to be applied to a file

type Entity interface {
	GetKind() string
	String() string
	Delete(s string) error
	Create(s string) error
	Apply(s string, safe bool) error
}

func entityIdentifier(s string) string {
	fs := strings.Split(s, ":")
	if len(fs) == 0 {
		return ""
	}

	return fs[0]
}

func RetryForDuration() (time.Duration, error) {
	s := os.Getenv(ENTITY_ENV_DEF_DELAY)
	if s != "" {
		// convert string to int64
		return time.ParseDuration(fmt.Sprintf("%s", s))
	}

	return time.ParseDuration("5s")
}

func RetryIntervalDuration() (time.Duration, error) {
	s := os.Getenv(ENTITY_ENV_DEF_INTERVAL)
	if s != "" {
		// convert string to int64
		return time.ParseDuration(fmt.Sprintf("%s", s))
	}

	return time.ParseDuration("300ms")
}
