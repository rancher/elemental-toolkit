package utils

import (
	"fmt"
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"io/ioutil"
	"os"
	"testing"
)

func TestDoCopyEmpty(t *testing.T) {
	RegisterTestingT(t)
	s, err := os.MkdirTemp("", "elemental")
	Expect(err).To(BeNil())
	defer os.RemoveAll(s)
	d, err := os.MkdirTemp("", "elemental")
	Expect(err).To(BeNil())
	defer os.RemoveAll(d)

	cfg := &v1.RunConfig{
		Device:    "",
		Target:    d,
		Source:    s,
		CloudInit: "",
	}

	err = DoCopy(cfg)
	Expect(err).To(BeNil())
}

func TestDoCopy(t *testing.T) {
	RegisterTestingT(t)
	s, err := os.MkdirTemp("", "elemental")
	Expect(err).To(BeNil())
	defer os.RemoveAll(s)
	d, err := os.MkdirTemp("", "elemental")
	Expect(err).To(BeNil())
	defer os.RemoveAll(d)

	for i := 0; i<5; i++ {
		_, _ = os.CreateTemp(s, "file*")
	}

	cfg := &v1.RunConfig{
		Device:    "",
		Target:    d,
		Source:    s,
		CloudInit: "",
	}


	err = DoCopy(cfg)
	Expect(err).To(BeNil())

	filesDest, err := ioutil.ReadDir(d)
	destNames := getNamesFromListFiles(filesDest)
	filesSource, err := ioutil.ReadDir(s)
	SourceNames := getNamesFromListFiles(filesSource)

	// Should be the same files in both dirs now
	Expect(destNames).To(Equal(SourceNames))
}


func TestDoCopyEmptyWithCloudInit(t *testing.T) {
	RegisterTestingT(t)
	testString := "In a galaxy far far away..."
	s, err := os.MkdirTemp("", "elemental")
	Expect(err).To(BeNil())
	defer os.RemoveAll(s)
	d, err := os.MkdirTemp("", "elemental")
	Expect(err).To(BeNil())
	defer os.RemoveAll(d)
	err = os.Mkdir(fmt.Sprintf("%s/oem", d), 0777)
	Expect(err).To(BeNil())

	cloudInit, err := os.CreateTemp("", "elemental*")
	_, err = cloudInit.WriteString(testString)
	Expect(err).To(BeNil())
	err = cloudInit.Close()
	Expect(err).To(BeNil())
	defer os.Remove(cloudInit.Name())

	cfg :=&v1.RunConfig{
		Target:    d,
		Source:    s,
		CloudInit: cloudInit.Name(),
	}

	err = DoCopy(cfg)
	Expect(err).To(BeNil())
	filesDest, err := ioutil.ReadDir(fmt.Sprintf("%s/oem", d))
	destNames := getNamesFromListFiles(filesDest)

	Expect(destNames).To(ContainElement("99_custom.yaml"))

	dest, err := ioutil.ReadFile(fmt.Sprintf("%s/oem/99_custom.yaml", d))
	Expect(dest).To(ContainSubstring(testString))

}

func getNamesFromListFiles(list []os.FileInfo) []string {
	var names []string
	for _,f := range list {
		names = append(names, f.Name())
	}
	return names
}
