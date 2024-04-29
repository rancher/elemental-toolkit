/*
Copyright © 2022 - 2024 SUSE LLC

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

package vm

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:revive
)

type PodmanRunCommand struct {
	sut        *SUT
	privileged bool
	image      string
	mounts     []VolumeMount
	entrypoint string
	command    string
}

type VolumeMount struct {
	from string
	to   string
}

func (p *PodmanRunCommand) Privileged() *PodmanRunCommand {
	p.privileged = true
	return p
}

func (p *PodmanRunCommand) WithMount(from, to string) *PodmanRunCommand {
	p.mounts = append(p.mounts, VolumeMount{from: from, to: to})
	return p
}

func (p *PodmanRunCommand) Run() (string, error) {
	priv := ""
	if p.privileged {
		priv = "--privileged"
	}

	mounts := []string{}
	for _, m := range p.mounts {
		mounts = append(mounts, fmt.Sprintf("-v %s:%s", m.from, m.to))
	}

	entrypoint := ""
	if p.entrypoint != "" {
		entrypoint = fmt.Sprintf("--entrypoint=%s", p.entrypoint)
	}

	cmd := fmt.Sprintf("podman run %s %s %s %s %s", priv, strings.Join(mounts, " "), entrypoint, p.image, p.command)
	By(fmt.Sprintf("Running podman command: '%s'", cmd))
	return p.sut.Command(cmd)
}
