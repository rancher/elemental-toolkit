//go:build ignore

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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: generate-tarballs [feature dir] [output dir]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		fmt.Println("Input and/or output dir is missing.")
		os.Exit(1)
	}

	inputDir := args[0]
	outputDir := args[1]

	dirs, err := os.ReadDir(inputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading dir: %s", err.Error())
		return
	}

	err = os.Mkdir(outputDir, os.ModeDir|os.ModePerm)
	if err != nil && !os.IsExist(err) {
		fmt.Fprintf(os.Stderr, "Error creating dir: %s", err.Error())
		return
	}

	for _, dir := range dirs {
		input := dir.Name()
		output := fmt.Sprintf("%s/%s.tar.gz", outputDir, dir.Name())

		fmt.Printf("Generate %s from %s\n", output, input)

		cmd := exec.Command("tar", "--sort=name", "--mtime=@1", "--owner=0", "--group=0", "--numeric-owner", "--pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime", "-C", inputDir, "-czvf", output, input)

		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tarball: %s\n", err.Error())
			fmt.Fprintf(os.Stderr, "Read: %s\n", string(out))
			return
		}
	}
}
