/*
Copyright © 2021 SUSE LLC

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

package v1

import (
	dockTypes "github.com/docker/docker/api/types"
	"github.com/docker/go-units"
	"github.com/mudler/luet/pkg/api/core/bus"
	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/helpers/docker"
)

type LuetInterface interface {
	Unpack(string, string, bool) error
}

type Luet struct {
	log               Logger
	context           *context.Context
	auth              *dockTypes.AuthConfig
	VerifyImageUnpack bool
}

func NewLuet(log Logger, context *context.Context, auth *dockTypes.AuthConfig, plugins ...string) *Luet {
	bus.Manager.Initialize(context, plugins...)
	if len(bus.Manager.Plugins) != 0 {
		log.Infof("Enabled plugins:")
		for _, p := range bus.Manager.Plugins {
			log.Infof("* %s (at %s)", p.Name, p.Executable)
		}
	}

	return &Luet{
		log:     log,
		context: context,
		auth:    auth,
	}
}

func (l Luet) Unpack(target string, image string, local bool) error {
	l.log.Infof("Unpacking docker image: %s", image)
	if !local {
		info, err := docker.DownloadAndExtractDockerImage(l.context, image, target, l.auth, l.VerifyImageUnpack)
		if err != nil {
			return err
		}
		l.log.Infof("Pulled: %s %s", info.Target.Digest, info.Name)
		l.log.Infof("Size: %s", units.BytesSize(float64(info.Target.Size)))
	} else {
		info, err := docker.ExtractDockerImage(l.context, image, target)
		if err != nil {
			return err
		}
		l.log.Infof("Size: %s", units.BytesSize(float64(info.Target.Size)))
	}
	return nil
}
