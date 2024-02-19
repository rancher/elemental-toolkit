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

package utils

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/rancher/yip/pkg/schema"
	"gopkg.in/yaml.v3"

	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
)

func onlyYAMLPartialErrors(er error) bool {
	if merr, ok := er.(*multierror.Error); ok {
		for _, e := range merr.Errors {
			// Skip partial unmarshalling errors
			// TypeError is throwed when it is possible to read the yaml partially
			// XXX: Seems errors.Is and errors.As are not working as expected here.
			// Even if the underlying type is yaml.TypeError.
			var d *yaml.TypeError
			if fmt.Sprintf("%T", e) != fmt.Sprintf("%T", d) {
				return false
			}
		}
	}
	return true
}

func checkYAMLError(cfg *v2.Config, allErrors, err error) error {
	if !onlyYAMLPartialErrors(err) {
		// here we absorb errors only if are related to YAML unmarshalling
		// As cmdline is parsed out as a yaml file
		allErrors = multierror.Append(allErrors, err)
	} else {
		cfg.Logger.Debug("/proc/cmdline parsing returned errors while unmarshalling. Ignoring as /proc/cmdline fields are turned to a YAML document, and partial failures are valid")
		cfg.Logger.Debug(err)
	}

	return allErrors
}

// RunStage will run yip
func RunStage(cfg *v2.Config, stage string, strict bool, cloudInitPaths ...string) error {
	var allErrors error

	cfg.Logger.Debugf("Cloud-init paths set to %v", cloudInitPaths)

	stageBefore := fmt.Sprintf("%s.before", stage)
	stageAfter := fmt.Sprintf("%s.after", stage)

	// Check if the cmdline has the cos.setup key and extract its value to run yip on that given uri
	cmdLineOut, err := cfg.Fs.ReadFile("/proc/cmdline")
	if err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	cmdLineArgs := strings.Split(string(cmdLineOut), " ")
	for _, line := range cmdLineArgs {
		if strings.Contains(line, "=") {
			lineSplit := strings.Split(line, "=")
			if lineSplit[0] == "cos.setup" {
				cloudInitPaths = append(cloudInitPaths, strings.TrimSpace(lineSplit[1]))
				cfg.Logger.Debugf("Found cos.setup stanza on cmdline with value %s", lineSplit[1])
			}
		}
	}

	// Run all stages for each of the default cloud config paths + extra cloud config paths
	if len(cloudInitPaths) > 0 {
		for _, s := range []string{stageBefore, stage, stageAfter} {
			err = cfg.CloudInitRunner.Run(s, filterNonExistingLocalURIs(cfg, cloudInitPaths...)...)
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
			}
		}
	}

	// Run stages encoded from /proc/cmdlines
	cfg.CloudInitRunner.SetModifier(schema.DotNotationModifier)

	for _, s := range []string{stageBefore, stage, stageAfter} {
		err = cfg.CloudInitRunner.Run(s, string(cmdLineOut))
		if err != nil {
			allErrors = checkYAMLError(cfg, allErrors, err)
		}
	}

	cfg.CloudInitRunner.SetModifier(nil)

	// We return error here only if we have been running in strict mode.
	// Cloud configs are being loaded and executed on a best-effort, so every step/config
	// gets a chance to be executed and error is being appended and reported.
	if allErrors != nil && !strict {
		cfg.Logger.Info("Some errors found but were ignored. Enable --strict mode to fail on those or --debug to see them in the log")
		cfg.Logger.Warn(allErrors)
		return nil
	}

	return allErrors
}

// filterNonExistingLocalURIs attempts to remove non existing local paths from the given URI slice.
// Returns the filtered slice.
func filterNonExistingLocalURIs(cfg *v2.Config, uris ...string) []string {
	filteredPaths := []string{}
	for _, cp := range uris {
		if local, _ := IsLocalURI(cp); local {
			if ok, _ := Exists(cfg.Fs, cp); !ok {
				cfg.Logger.Debugf("Ignoring cloud-init local config path %s. Could not find it.", cp)
				continue
			}
		}
		filteredPaths = append(filteredPaths, cp)
	}
	return filteredPaths
}
