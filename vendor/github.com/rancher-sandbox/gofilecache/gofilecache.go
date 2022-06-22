// Copyright 2017 The Go Authors, SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gofilecache

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

const cacheREADME = `This directory holds a cache directory tree.
This has been created using github.com/rancher-sandbox/gofilecache/
`

func InitCache(cacheDir string) *Cache {
	if err := os.MkdirAll(cacheDir, 0777); err != nil {
		log.Fatalf("failed to initialize build cache at %s: %s\n", cacheDir, err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "README")); err != nil {
		// Best effort.
		ioutil.WriteFile(filepath.Join(cacheDir, "README"), []byte(cacheREADME), 0666)
	}

	c, err := Open(cacheDir)
	if err != nil {
		log.Fatalf("failed to initialize build cache at %s: %s\n", cacheDir, err)
	}
	return c
}

var ()
