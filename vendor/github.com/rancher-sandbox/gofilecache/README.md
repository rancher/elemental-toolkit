# GOFILECACHE

This is a simple filecache derived from golangs buildcache.

## How to use

```golang
package main

import (
  "crypto/sha512"
  "io/ioutil"
  "os"
  "github.com/rancher-sandbox/gofilecache"
)

func main() {
  // Initialise cache
  cache := gofilecache.InitCache("temp/")
  // pick an example textfile to add to the cache
  testFile := "/usr/lib/os-release"
  file, _ := os.Open(testFile)
  defer file.Close()
  // generate a hash under which the entry can be found in the cache
  // for simplicity we use the filename here
  actionID := sha512.Sum512([]byte("os-release"))

  // store the files contents to the cache
  cache.Put(actionID, file)

  // retrieve the filename from the cache
  fileName, _, _ := cache.GetFile(actionID)

  // get the files contents
  _, _ = ioutil.ReadFile(fileName)
}

```
