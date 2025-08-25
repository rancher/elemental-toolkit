//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package types

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultChroot             = "/"
	DefaultCacheOnly          = false
	DefaultEnableNetworkFetch = false
)

var (
	DefaultCachePath = getCachePath()
)

func getCachePath() string {
	hdir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed getting os.UserHomeDir(): %v", err)
		return ""
	}
	return filepath.Join(hdir, ".cache", "pci.ids")
}
