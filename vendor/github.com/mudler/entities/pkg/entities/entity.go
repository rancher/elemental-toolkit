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
	"os"
	"strconv"
	"strings"
)

const (
	ENTITY_ENV_DEF_GROUPS        = "ENTITY_DEFAULT_GROUPS"
	ENTITY_ENV_DEF_PASSWD        = "ENTITY_DEFAULT_PASSWD"
	ENTITY_ENV_DEF_SHADOW        = "ENTITY_DEFAULT_SHADOW"
	ENTITY_ENV_DEF_GSHADOW       = "ENTITY_DEFAULT_GSHADOW"
	ENTITY_ENV_DEF_DYNAMIC_RANGE = "ENTITY_DYNAMIC_RANGE"
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

func DynamicRange() (int, int) {
	// Follow Gentoo way
	uid_start := 999
	uid_end := 500

	// Environment variable must be in the format: <minUid> + '-' + <maxUid>
	env := os.Getenv(ENTITY_ENV_DEF_DYNAMIC_RANGE)
	if env != "" {
		ranges := strings.Split(env, "-")
		if len(ranges) == 2 {
			minUid, err := strconv.Atoi(ranges[0])
			if err != nil {
				// Ignore error
				goto end
			}
			maxUid, err := strconv.Atoi(ranges[1])
			if err != nil {
				// ignore error
				goto end
			}

			if minUid < maxUid && minUid >= 0 && minUid < 65534 && maxUid > 0 && maxUid < 65534 {
				uid_start = maxUid
				uid_end = minUid
			}
		}
	}
end:

	return uid_start, uid_end
}
