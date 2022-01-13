/*
Copyright © 2022 SUSE LLC

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

package utils

import (
	"fmt"
	"github.com/mudler/yip/pkg/schema"
	"github.com/rancher-sandbox/elemental-cli/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/afero"
	"strings"
)

// RunStage will run yip
func RunStage(stage string, cfg *v1.RunConfig) error {
	var cmdLineYipUri string
	var FinalCloudInitPaths []string
	CloudInitPaths := constants.GetCloudInitPaths()

	// Check if we have extra cloud init
	// This requires fixing the env vars, otherwise it wont work
	if cfg.CloudInitPaths != "" {
		cfg.Logger.Debugf("Adding extra paths: %s", cfg.CloudInitPaths)
		extraCloudInitPathsSplit := strings.Split(cfg.CloudInitPaths, " ")
		CloudInitPaths = append(CloudInitPaths, extraCloudInitPathsSplit...)
	}

	// Cleanup paths. Check if they exist and add them to the final list to avoid failures on non-existing paths
	for _, path := range CloudInitPaths {
		exists, err := afero.Exists(cfg.Fs, path)
		if exists && err == nil {
			FinalCloudInitPaths = append(FinalCloudInitPaths, path)
		} else {
			cfg.Logger.Debugf("Skipping path %s as it doesnt exists or cant access it", path)
		}
	}

	stageBefore := fmt.Sprintf("%s.before", stage)
	stageAfter := fmt.Sprintf("%s.after", stage)

	// Check if the cmdline has the cos.setup key and extract its value to run yip on that given uri
	cmdLineOut, err := cfg.Runner.Run("cat", "/proc/cmdline")
	if err != nil {
		return err
	}
	cmdLine := strings.Split(string(cmdLineOut), " ")
	for _, line := range cmdLine {
		if strings.Contains(line, "=") {
			lineSplit := strings.Split(line, "=")
			if lineSplit[0] == "cos.setup" {
				cmdLineYipUri = lineSplit[1]
				cfg.Logger.Debugf("Found cos.setup stanza on cmdline with value %s", cmdLineYipUri)
			}
		}
	}

	// Run the stage.before if cmdline contains the cos.setup stanza
	if cmdLineYipUri != "" {
		cmdLineArgs := []string{cmdLineYipUri}
		err := cfg.CloudInitRunner.Run(stageBefore, cmdLineArgs...)
		if err != nil {
			return err
		}
	}

	// Run all stages for each of the default cloud config paths + extra cloud config paths
	err = cfg.CloudInitRunner.Run(stageBefore, FinalCloudInitPaths...)
	if err != nil {
		return err
	}
	err = cfg.CloudInitRunner.Run(stage, FinalCloudInitPaths...)
	if err != nil {
		return err
	}
	err = cfg.CloudInitRunner.Run(stageAfter, FinalCloudInitPaths...)
	if err != nil {
		return err
	}

	// Run the stage.after if cmdline contains the cos.setup stanza
	if cmdLineYipUri != "" {
		cmdLineArgs := []string{cmdLineYipUri}
		err := cfg.CloudInitRunner.Run(stageAfter, cmdLineArgs...)
		if err != nil {
			return err
		}
	}

	cfg.CloudInitRunner.SetModifier(schema.DotNotationModifier)
	err = cfg.CloudInitRunner.Run(stageBefore, string(cmdLineOut))
	if err != nil {
		return err
	}
	err = cfg.CloudInitRunner.Run(stage, string(cmdLineOut))
	if err != nil {
		return err
	}
	err = cfg.CloudInitRunner.Run(stageAfter, string(cmdLineOut))
	if err != nil {
		return err
	}
	return nil
}
