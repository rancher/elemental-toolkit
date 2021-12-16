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

package elemental

import (
	"errors"
	"fmt"
	. "github.com/onsi/gomega"
	cnst "github.com/rancher-sandbox/elemental-cli/pkg/constants"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"io/ioutil"
	"k8s.io/mount-utils"
	"os"
	"testing"
)

var printOutput = `BYT;
/dev/loop0:50593792s:loopback:512:512:gpt:Loopback device:;`
var partTmpl = `
%d:%ss:%ss:2048s:ext4::type=83;`

func TestDoCopyEmpty(t *testing.T) {
	RegisterTestingT(t)
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
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
		Logger:    logger,
	}

	c := Elemental{config: cfg}

	err = c.CopyCos()
	Expect(err).To(BeNil())
}

func TestDoCopy(t *testing.T) {
	RegisterTestingT(t)
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
	s, err := os.MkdirTemp("", "elemental")
	Expect(err).To(BeNil())
	defer os.RemoveAll(s)
	d, err := os.MkdirTemp("", "elemental")
	Expect(err).To(BeNil())
	defer os.RemoveAll(d)

	for i := 0; i < 5; i++ {
		_, _ = os.CreateTemp(s, "file*")
	}

	cfg := &v1.RunConfig{
		Device:    "",
		Target:    d,
		Source:    s,
		CloudInit: "",
		Logger:    logger,
	}

	c := Elemental{config: cfg}
	err = c.CopyCos()
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
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
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

	cfg := &v1.RunConfig{
		Target:    d,
		Source:    s,
		CloudInit: cloudInit.Name(),
		Logger:    logger,
	}

	c := Elemental{config: cfg}
	err = c.CopyCos()
	Expect(err).To(BeNil())
	err = c.CopyCloudConfig()
	Expect(err).To(BeNil())
	filesDest, err := ioutil.ReadDir(fmt.Sprintf("%s/oem", d))
	destNames := getNamesFromListFiles(filesDest)

	Expect(destNames).To(ContainElement("99_custom.yaml"))

	dest, err := ioutil.ReadFile(fmt.Sprintf("%s/oem/99_custom.yaml", d))
	Expect(dest).To(ContainSubstring(testString))

}

func TestSelinuxRelabel(t *testing.T) {
	// I cant seem to mock exec.LookPath so it will always fail tor un due setfiles not being in the system :/
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	cfg := &v1.RunConfig{Target: "/", Fs: fs}
	c := Elemental{config: cfg}
	// This is actually failing but not sure we should return an error
	Expect(c.SelinuxRelabel(true)).ToNot(BeNil())
	fs = afero.NewMemMapFs()
	_, _ = fs.Create("/etc/selinux/targeted/contexts/files/file_contexts")
	Expect(c.SelinuxRelabel(false)).To(BeNil())
}

func TestCheckFormat(t *testing.T) {
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	cfg := &v1.RunConfig{Target: "/", Fs: fs}
	cos := NewElemental(cfg)
	err := cos.CheckNoFormat()
	Expect(err).To(BeNil())
}

func TestCheckNoFormat(t *testing.T) {
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	runner := v1mock.FakeRunner{}
	cfg := &v1.RunConfig{Target: "/", Fs: fs, NoFormat: true, Runner: &runner}
	cos := NewElemental(cfg)
	err := cos.CheckNoFormat()
	Expect(err).To(BeNil())
}

// TestCheckNoFormatWithLabel tests when we set no format but labels exists for active/passive partition
func TestCheckNoFormatWithLabel(t *testing.T) {
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	logger := v1.NewNullLogger()
	runner := v1mock.NewTestRunnerV2()
	runner.ReturnValue = []byte("/dev/fake")
	cfg := &v1.RunConfig{Target: "/", Fs: fs, NoFormat: true, Runner: runner, Logger: logger}
	cos := NewElemental(cfg)
	err := cos.CheckNoFormat()
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("There is already an active deployment"))
}

// TestCheckNoFormatWithLabel tests when we set no format but labels exists for active/passive partition AND we set the force flag
func TestCheckNoFormatWithLabelAndForce(t *testing.T) {
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	logger := v1.NewNullLogger()
	runner := v1mock.NewTestRunnerV2()
	runner.ReturnValue = []byte("/dev/fake")
	cfg := &v1.RunConfig{Target: "/", Fs: fs, NoFormat: true, Force: true, Runner: runner, Logger: logger}
	cos := NewElemental(cfg)
	err := cos.CheckNoFormat()
	Expect(err).To(BeNil())
}

func TestPartitionAndFormatDevice(t *testing.T) {
	RegisterTestingT(t)
	runner := v1mock.NewTestRunnerV2()
	fs := afero.NewMemMapFs()
	conf := v1.NewRunConfig(
		v1.WithLogger(v1.NewNullLogger()),
		v1.WithRunner(runner),
		v1.WithFs(fs),
		v1.WithMounter(&mount.FakeMounter{}),
	)
	fs.Create(cnst.EfiDevice)
	conf.SetupStyle()
	dev := part.NewDisk(
		"/some/device",
		part.WithRunner(runner),
		part.WithFS(fs),
		part.WithLogger(conf.Logger),
	)
	var partNum, devNum int
	printOut := printOutput

	cmds := [][]string{
		{
			"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
			"mklabel", "gpt",
		}, {
			"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
			"mkpart", "p.grub", "vfat", "2048", "133119", "set", "1", "esp", "on",
		}, {"mkfs.vfat", "-i", "COS_GRUB", "/some/device1"}, {
			"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
			"mkpart", "p.oem", "ext4", "133120", "264191",
		}, {
			"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
			"mkpart", "p.state", "ext4", "264192", "31721471",
		}, {
			"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
			"mkpart", "p.recovery", "ext4", "31721472", "48498687",
		}, {
			"parted", "--script", "--machine", "--", "/some/device", "unit", "s",
			"mkpart", "p.persistent", "ext4", "48498688", "100%",
		}, {"mkfs.ext4", "-L", "COS_OEM", "/some/device2"},
		{"mkfs.ext4", "-L", "COS_STATE", "/some/device3"},
		{"mkfs.ext4", "-L", "COS_RECOVERY", "/some/device4"},
		{"mkfs.ext4", "-L", "COS_PERSISTENT", "/some/device5"},
	}

	runFunc := func(cmd string, args ...string) ([]byte, error) {
		switch cmd {
		case "parted":
			idx := 0
			for i, arg := range args {
				if arg == "mkpart" {
					idx = i
					break
				}
			}
			if idx > 0 {
				partNum++
				printOut += fmt.Sprintf(partTmpl, partNum, args[idx+3], args[idx+4])
			}
			return []byte(printOut), nil
		case "lsblk":
			devNum++
			return []byte(fmt.Sprintf("/some/device%d part", devNum)), nil
		default:
			return []byte{}, nil
		}
	}
	runner.SideEffect = runFunc
	el := NewElemental(conf)
	err := el.PartitionAndFormatDevice(dev)
	Expect(err).To(BeNil())
	Expect(runner.MatchMilestones(cmds)).To(BeNil())
}

func TestPartitionAndFormatDeviceErrors(t *testing.T) {
	RegisterTestingT(t)
	runner := v1mock.NewTestRunnerV2()
	fs := afero.NewMemMapFs()
	conf := v1.NewRunConfig(
		v1.WithLogger(v1.NewNullLogger()),
		v1.WithRunner(runner),
		v1.WithFs(fs),
		v1.WithMounter(&mount.FakeMounter{}),
	)
	fs.Create(cnst.EfiDevice)
	conf.SetupStyle()
	dev := part.NewDisk(
		"/some/device",
		part.WithRunner(runner),
		part.WithFS(fs),
		part.WithLogger(conf.Logger),
	)
	var partNum, devNum, errPart int
	var printOut, errFormat string

	runFunc := func(cmd string, args ...string) ([]byte, error) {
		switch cmd {
		case "parted":
			idx := 0
			for i, arg := range args {
				if arg == "mkpart" {
					idx = i
					break
				}
			}
			if idx > 0 {
				partNum++
				printOut += fmt.Sprintf(partTmpl, partNum, args[idx+3], args[idx+4])
				if errPart == partNum {
					return []byte{}, errors.New("Failure")
				}
			}
			return []byte(printOut), nil
		case "lsblk":
			devNum++
			return []byte(fmt.Sprintf("/some/device%d part", devNum)), nil
		case "mkfs.ext4", "mkfs.vfat":
			if args[1] == errFormat {
				return []byte{}, errors.New("Failure")
			}
			return []byte{}, nil
		default:
			return []byte{}, nil
		}
	}
	runner.SideEffect = runFunc
	el := NewElemental(conf)

	// Fails efi partition
	errPart, partNum, devNum, errFormat, printOut = 1, 0, 0, "COS_GRUB", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(partNum).To(Equal(errPart))

	// Fails efi format
	errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_GRUB", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(devNum).To(Equal(1))

	// Fails oem partition
	errPart, partNum, devNum, errFormat, printOut = 2, 0, 0, "", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(partNum).To(Equal(errPart))

	// Fails state partition
	errPart, partNum, devNum, errFormat, printOut = 3, 0, 0, "", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(partNum).To(Equal(errPart))

	// Fails recovery partition
	errPart, partNum, devNum, errFormat, printOut = 4, 0, 0, "", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(partNum).To(Equal(errPart))

	// Fails persistent partition
	errPart, partNum, devNum, errFormat, printOut = 5, 0, 0, "", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(partNum).To(Equal(errPart))

	// Fails oem format
	errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_OEM", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(devNum).To(Equal(2))

	// Fails state format
	errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_STATE", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(devNum).To(Equal(3))

	// Fails recovery format
	errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_RECOVERY", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(devNum).To(Equal(4))

	// Fails persistent format
	errPart, partNum, devNum, errFormat, printOut = 0, 0, 0, "COS_PERSISTENT", printOutput
	Expect(el.PartitionAndFormatDevice(dev)).NotTo(BeNil())
	Expect(devNum).To(Equal(5))
}

func getNamesFromListFiles(list []os.FileInfo) []string {
	var names []string
	for _, f := range list {
		names = append(names, f.Name())
	}
	return names
}
