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
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/schema"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"gopkg.in/yaml.v3"
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

func checkYAMLError(cfg *v1.Config, allErrors, err error) error {
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
func RunStage(cfg *v1.Config, stage string, strict bool, cloudInitPaths ...string) error {
	var cmdLineYipURI string
	var allErrors error

	cloudInitPaths = append(constants.GetCloudInitPaths(), cloudInitPaths...)
	cfg.Logger.Debugf("Cloud-init paths set to %v", cloudInitPaths)

	// Make sure cloud init path specified are existing in the system
	for _, cp := range cloudInitPaths {
		err := MkdirAll(cfg.Fs, cp, constants.DirPerm)
		if err != nil {
			cfg.Logger.Debugf("Failed creating cloud-init config path: %s %s", cp, err.Error())
		}
	}

	stageBefore := fmt.Sprintf("%s.before", stage)
	stageAfter := fmt.Sprintf("%s.after", stage)

	// Check if the cmdline has the cos.setup key and extract its value to run yip on that given uri
	cmdLineOut, err := cfg.Fs.ReadFile("/proc/cmdline")
	if err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	cmdLine := strings.Split(string(cmdLineOut), " ")
	for _, line := range cmdLine {
		if strings.Contains(line, "=") {
			lineSplit := strings.Split(line, "=")
			if lineSplit[0] == "cos.setup" {
				cmdLineYipURI = lineSplit[1]
				cfg.Logger.Debugf("Found cos.setup stanza on cmdline with value %s", cmdLineYipURI)
			}
		}
	}

	// Run all stages for each of the default cloud config paths + extra cloud config paths
	for _, s := range []string{stageBefore, stage, stageAfter} {
		err = cfg.CloudInitRunner.Run(s, cloudInitPaths...)
		if err != nil {
			allErrors = multierror.Append(allErrors, err)
		}
	}

	// Run the stages if cmdline contains the cos.setup stanza
	if cmdLineYipURI != "" {
		cmdLineArgs := []string{cmdLineYipURI}
		for _, s := range []string{stageBefore, stage, stageAfter} {
			err = cfg.CloudInitRunner.Run(s, cmdLineArgs...)
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
