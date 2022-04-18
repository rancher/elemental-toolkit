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

package cmd

import (
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
)

func executeCommandC(cmd *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	// Set args to command
	cmd.SetArgs(args)
	// store old stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	// Change stdout to our pipe
	os.Stdout = w
	// run the command
	c, err = cmd.ExecuteC()
	if err != nil {
		// Remember to restore stdout!
		os.Stdout = oldStdout
		return nil, "", err
	}
	err = w.Close()
	if err != nil {
		// Remember to restore stdout!
		os.Stdout = oldStdout
		return nil, "", err
	}
	// Read output from our pipe
	out, _ := ioutil.ReadAll(r)
	// restore stdout
	os.Stdout = oldStdout

	return c, string(out), nil
}
