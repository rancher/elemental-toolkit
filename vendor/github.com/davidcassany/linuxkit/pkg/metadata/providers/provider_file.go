package providers

import (
	"io/ioutil"
	"os"
)

type FileProvider string

func (p FileProvider) String() string {
	return string(p)
}

func (p FileProvider) Probe() bool {
	_, err := os.Stat(string(p))
	return err == nil
}

func (p FileProvider) Extract() ([]byte, error) {
	return ioutil.ReadFile(string(p))
}
