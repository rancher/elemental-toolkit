/*
Copyright © 2022 - 2025 SUSE LLC

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

package mocks

import (
	"errors"
	"fmt"

	"github.com/rancher/yip/pkg/schema"
)

type FakeCloudInitRunner struct {
	ExecStages []string
	Error      bool
	RenderErr  bool
	stageArgs  map[string][]string
}

func appendIfMissing(slice []string, item string) []string {
	for _, it := range slice {
		if it == item {
			return slice
		}
	}
	return append(slice, item)
}

func (ci *FakeCloudInitRunner) Run(stage string, args ...string) error {
	if ci.stageArgs == nil {
		ci.stageArgs = map[string][]string{}
	}

	// keeps a list of unique arguments passed to each stage
	for _, arg := range args {
		ci.stageArgs[stage] = appendIfMissing(ci.stageArgs[stage], arg)
	}

	ci.ExecStages = append(ci.ExecStages, stage)
	if ci.Error {
		return errors.New("cloud init failure")
	}
	return nil
}

func (ci *FakeCloudInitRunner) SetModifier(_ schema.Modifier) {
}

func (ci *FakeCloudInitRunner) GetStageArgs(stage string) []string {
	return ci.stageArgs[stage]
}

func (ci *FakeCloudInitRunner) CloudInitFileRender(_ string, _ *schema.YipConfig) error {
	if ci.RenderErr {
		return fmt.Errorf("failed redering yip file")
	}
	return nil
}
