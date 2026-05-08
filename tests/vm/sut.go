/*
Copyright © 2022 - 2026 SUSE LLC

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

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bramvdbogaerde/go-scp"

	. "github.com/onsi/ginkgo/v2" //nolint:revive
	. "github.com/onsi/gomega"    //nolint:revive
	"github.com/pkg/errors"
	ssh "golang.org/x/crypto/ssh"
)

const (
	Passive     = "passive"
	Active      = "active"
	Recovery    = "recovery"
	LiveCD      = "liveCD"
	UnknownBoot = "unknown"

	TimeoutRawDiskTest = 600 // Timeout to connect for recovery_raw_disk_test

	Ext2      = "ext2"
	Ext3      = "ext3"
	Ext4      = "ext4"
	Elemental = "elemental"
)

// DiskLayout is the struct that contains the disk output from lsblk
type DiskLayout struct {
	BlockDevices []PartitionEntry `json:"blockdevices"`
}

// PartitionEntry represents a partition entry
type PartitionEntry struct {
	Label  string `json:"label,omitempty"`
	Size   int    `json:"size,omitempty"`
	FsType string `json:"fstype,omitempty"`
}

func (d DiskLayout) GetPartition(label string) (PartitionEntry, error) {
	for _, device := range d.BlockDevices {
		if device.Label == label {
			return device, nil
		}
	}
	return PartitionEntry{}, nil
}

type SUT struct {
	Host          string
	Username      string
	Password      string
	SSHKey        []byte
	Timeout       int
	artifactsRepo string
	TestVersion   string
	CDLocation    string
	MachineID     string
	VMPid         int
}

func NewSUT() *SUT {
	var sshKey []byte
	var err error

	user := os.Getenv("COS_USER")
	if user == "" {
		user = "root"
	}

	sshKeyFile := os.Getenv("COS_SSHKEY")
	if sshKeyFile == "" {
		sshKeyFile = "../assets/testkey"
	}

	sshKey, err = os.ReadFile(sshKeyFile)
	if err != nil {
		fmt.Printf("failed reading ssh key file: %s\n", sshKeyFile)
	}

	pass := os.Getenv("COS_PASS")
	if pass == "" {
		pass = Elemental
	}

	host := os.Getenv("COS_HOST")
	if host == "" {
		host = "127.0.0.1:2222"
	}

	var vmPid int
	vmPidStr := os.Getenv("VM_PID")
	value, err := strconv.Atoi(vmPidStr)
	if err == nil {
		By(fmt.Sprintf("Underlaying VM pid is set to: %d", value))
		vmPid = value
	}

	testVersion := os.Getenv("TEST_VERSION")
	if testVersion == "" {
		testVersion = "0.8.14-1"
	}

	var timeout = 180
	valueStr := os.Getenv("COS_TIMEOUT")
	value, err = strconv.Atoi(valueStr)
	if err == nil {
		timeout = value
	}

	return &SUT{
		Host:          host,
		Username:      user,
		Password:      pass,
		SSHKey:        sshKey,
		MachineID:     "test",
		Timeout:       timeout,
		artifactsRepo: "",
		TestVersion:   testVersion,
		CDLocation:    "",
		VMPid:         vmPid,
	}
}

func (s *SUT) ChangeBoot(b string) error {
	var bootEntry string

	switch b {
	case Active:
		bootEntry = "active"
	case Passive:
		bootEntry = "1"
	case Recovery:
		bootEntry = "recovery"
	}

	cmd := "grub2-editenv"
	_, err := s.command(fmt.Sprintf("which %s", cmd))
	if err != nil {
		cmd = "grub-editenv"
	}

	_, err = s.command(fmt.Sprintf("%s /oem/grubenv set saved_entry=%s", cmd, bootEntry))
	Expect(err).ToNot(HaveOccurred())

	return nil
}

func (s *SUT) ChangeBootOnce(b string) error {
	var bootEntry string

	switch b {
	case Active:
		bootEntry = "active"
	case Passive:
		bootEntry = "1"
	case Recovery:
		bootEntry = "recovery"
	}

	cmd := "grub2-editenv"
	_, err := s.command(fmt.Sprintf("which %s", cmd))
	if err != nil {
		cmd = "grub-editenv"
	}

	_, err = s.command(fmt.Sprintf("%s /oem/grubenv set next_entry=%s", cmd, bootEntry))
	Expect(err).ToNot(HaveOccurred())

	return nil
}

// Reset runs reboots cOS into Recovery and runs elemental reset.
// It will boot back the system from the Active partition afterwards
func (s *SUT) Reset() {
	if s.BootFrom() != Recovery {
		By("Reboot to recovery before reset")
		err := s.ChangeBootOnce(Recovery)
		Expect(err).ToNot(HaveOccurred())
		s.Reboot()
		s.EventuallyBootedFrom(Recovery)
	}

	By("Running elemental reset")
	out, err := s.command("elemental reset")
	Expect(err).ToNot(HaveOccurred())
	Expect(out).Should(ContainSubstring("Reset"))

	By("Reboot to active after elemental reset")
	s.Reboot()
	ExpectWithOffset(1, s.BootFrom()).To(Equal(Active))
}

// BootFrom returns the booting partition of the SUT
func (s *SUT) BootFrom() string {
	out, err := s.command("cat /proc/cmdline")
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	switch {
	case strings.Contains(out, "active"):
		return Active
	case strings.Contains(out, "passive"):
		return Passive
	case strings.Contains(out, "recovery"):
		return Recovery
	case strings.Contains(out, "live:CDLABEL"):
		return LiveCD
	default:
		return UnknownBoot
	}
}

func (s *SUT) EventuallyBootedFrom(image string) {
	Eventually(func() error {
		actual := s.BootFrom()
		if actual != image {
			return fmt.Errorf("expected boot from %s, actual %s", image, actual)
		}

		return nil
	}, time.Duration(60)*time.Second, time.Duration(10)*time.Second).ShouldNot(HaveOccurred())
}

func (s *SUT) GetOSRelease(ss string) string {
	out, err := s.Command(fmt.Sprintf("source /etc/os-release && echo $%s", ss))
	Expect(err).ToNot(HaveOccurred())
	Expect(out).ToNot(Equal(""))

	return strings.TrimSpace(out)
}

func (s *SUT) GetArch() string {
	out, err := s.Command("uname -p")
	Expect(err).ToNot(HaveOccurred())
	Expect(out).ToNot(Equal(""))

	return strings.TrimSpace(out)
}

func (s *SUT) EventuallyConnects(t ...int) {
	dur := s.Timeout
	if len(t) > 0 {
		dur = t[0]
	}
	Eventually(func() (string, error) {
		if !s.IsVMRunning() {
			return "", StopTrying("Underlaying VM is no longer running!")
		}
		return s.command("echo -n ping")
	}, time.Duration(time.Duration(dur)*time.Second), time.Duration(5*time.Second)).Should(Equal("ping"))
}

func (s *SUT) EventuallyDisconnects(t ...int) {
	dur := s.Timeout
	if len(t) > 0 {
		dur = t[0]
	}
	s.EventuallyConnects(10)
	Eventually(func() (string, error) {
		if !s.IsVMRunning() {
			return "", StopTrying("Underlaying VM is no longer running!")
		}
		out, _ := s.command("sleep 30 && echo -n ping")
		return out, nil
	}, time.Duration(time.Duration(dur)*time.Second), time.Duration(2*time.Second)).ShouldNot(Equal("ping"))
}

func (s *SUT) IsVMRunning() bool {
	if s.VMPid <= 0 {
		// Can't check without a pid, assume it is always running
		return true
	}
	proc, err := os.FindProcess(s.VMPid)
	if err != nil || proc == nil {
		return false
	}

	// On Unix FindProcess does not error out if the process does not
	// exist, so we send a test signal
	return proc.Signal(syscall.Signal(0)) == nil
}

func (s *SUT) NewPodmanRunCommand(image, command string) *PodmanRunCommand {
	return &PodmanRunCommand{
		tlsVerify:  true,
		sut:        s,
		image:      image,
		entrypoint: "/bin/bash",
		command:    command,
		mounts: []VolumeMount{
			{
				from: "/tmp",
				to:   "/tmp",
			},
		},
	}
}

// Command sends a command to the SUIT and waits for reply
func (s *SUT) Command(cmd string) (string, error) {
	if !s.IsVMRunning() {
		return "", fmt.Errorf("VM is not running, doesn't make sense running any command")
	}
	return s.command(cmd)
}

func (s *SUT) command(cmd string) (string, error) {
	client, err := s.connectToHost()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = client.Close()
	}()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = session.Close()
	}()

	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(out), errors.Wrap(err, string(out))
	}

	return string(out), err
}

// Reboot reboots the system under test
func (s *SUT) Reboot(t ...int) {
	By("Reboot")
	_, _ = s.command("reboot")
	time.Sleep(10 * time.Second)
	s.EventuallyConnects(t...)
}

func (s *SUT) clientConfig() *ssh.ClientConfig {
	var signer ssh.Signer
	var err error
	auths := []ssh.AuthMethod{}

	if s.SSHKey != nil {
		signer, err = ssh.ParsePrivateKey(s.SSHKey)
		if err != nil {
			log.Fatalf("unable to parse private key: %v", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}

	sshConfig := &ssh.ClientConfig{
		User:            s.Username,
		Auth:            append(auths, ssh.Password(s.Password)),
		Timeout:         15 * time.Second, // max time to establish connection
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return sshConfig
}

func (s *SUT) SendFile(src, dst, permission string) error {
	sshConfig := s.clientConfig()
	scpClient := scp.NewClientWithTimeout(s.Host, sshConfig, 10*time.Second)
	defer scpClient.Close()

	if err := scpClient.Connect(); err != nil {
		return err
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}

	defer scpClient.Close()
	defer func() {
		_ = f.Close()
	}()

	return scpClient.CopyFile(context.TODO(), f, dst, permission)
}

func (s *SUT) connectToHost() (*ssh.Client, error) {
	sshConfig := s.clientConfig()

	client, err := SSHDialTimeout("tcp", s.Host, sshConfig, sshConfig.Timeout)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// GatherAllLogs will try to gather as much info from the system as possible, including services, dmesg and os related info
func (s SUT) GatherAllLogs() {
	services := []string{
		"elemental-setup-boot",
		"elemental-setup-fs",
		"elemental-setup-initramfs",
		"elemental-setup-network",
		"elemental-setup-reconcile",
		"elemental-setup-rootfs",
		"elemental-immutable-rootfs",
	}

	logFiles := []string{
		"/tmp/elemental.log",
	}

	// services
	for _, ser := range services {
		out, err := s.command(fmt.Sprintf("journalctl -u %s -o short-iso >> /tmp/%s.log", ser, ser))
		if err != nil {
			fmt.Printf("Error getting journal for service %s: %s\n", ser, err.Error())
			fmt.Printf("Output from command: %s\n", out)
		}
		s.GatherLog(fmt.Sprintf("/tmp/%s.log", ser))
	}

	// log files
	for _, file := range logFiles {
		s.GatherLog(file)
	}

	// dmesg
	out, err := s.command("dmesg > /tmp/dmesg")
	if err != nil {
		fmt.Printf("Error getting dmesg : %s\n", err.Error())
		fmt.Printf("Output from command: %s\n", out)
	}
	s.GatherLog("/tmp/dmesg")

	// grab full journal
	out, err = s.command("journalctl -o short-iso > /tmp/journal.log")
	if err != nil {
		fmt.Printf("Error getting full journalctl info : %s\n", err.Error())
		fmt.Printf("Output from command: %s\n", out)
	}
	s.GatherLog("/tmp/journal.log")

	// uname
	out, err = s.command("uname -a > /tmp/uname.log")
	if err != nil {
		fmt.Printf("Error getting uname info : %s\n", err.Error())
		fmt.Printf("Output from command: %s\n", out)
	}
	s.GatherLog("/tmp/uname.log")

	// disk info
	out, err = s.command("lsblk -a >> /tmp/disks.log")
	if err != nil {
		fmt.Printf("Error getting disk info : %s\n", err.Error())
		fmt.Printf("Output from command: %s\n", out)
	}
	out, err = s.command("blkid >> /tmp/disks.log")
	if err != nil {
		fmt.Printf("Error getting disk info : %s\n", err.Error())
		fmt.Printf("Output from command: %s\n", out)
	}
	s.GatherLog("/tmp/disks.log")

	// Grab users
	s.GatherLog("/etc/passwd")
	// Grab system info
	s.GatherLog("/etc/os-release")

}

// GatherLog will try to scp the given log from the machine to a local file
func (s SUT) GatherLog(logPath string) {
	sshConfig := s.clientConfig()
	scpClient := scp.NewClientWithTimeout(s.Host, sshConfig, 20*time.Second)

	err := scpClient.Connect()
	if err != nil {
		scpClient.Close()
		fmt.Println("Couldn't establish a connection to the remote server ", err)
		return
	}

	baseName := filepath.Base(logPath)
	_ = os.Mkdir("logs", 0755)

	f, _ := os.Create(fmt.Sprintf("logs/%s", baseName))
	// Close the file after it has been copied
	// Close client connection after the file has been copied
	defer scpClient.Close()
	defer func() {
		_ = f.Close()
	}()

	err = scpClient.CopyFromRemote(context.TODO(), f, logPath)

	if err != nil {
		fmt.Printf("Error while copying file: %s\n", err.Error())
		return
	}
	// Change perms so its world readable
	_ = os.Chmod(fmt.Sprintf("logs/%s", baseName), 0666)

}

// EmptyDisk will try to trash the disk given so on reboot the disk is empty and we are forced to use the cd to boot
// used mainly for installer testing booting from iso
func (s *SUT) EmptyDisk(disk string) {
	By(fmt.Sprintf("Trashing %s to restore VM to a blank state", disk))
	_, _ = s.Command(fmt.Sprintf("wipefs -af %s*", disk))
	_, _ = s.Command("sync")
	_, _ = s.Command("sleep 5")
}

// SetCDLocation gets the location of the iso attached to the vbox vm and stores it for later remount
func (s *SUT) SetCDLocation() {
	By("Store CD location")
	out, err := exec.Command("bash", "-c", "VBoxManage list dvds|grep Location|cut -d ':' -f 2|xargs").CombinedOutput()
	Expect(err).To(BeNil())
	s.CDLocation = strings.TrimSpace(string(out))
}

// EjectCD force removes the DVD so we can boot from disk directly on EFI VMs
func (s *SUT) EjectCD() {
	// first store the cd location
	s.SetCDLocation()
	By("Ejecting the CD")
	_, err := exec.Command("bash", "-c", "VBoxManage storageattach 'test' --storagectl 'sata controller' --port 1 --device 0 --type dvddrive --medium emptydrive --forceunmount").CombinedOutput()
	Expect(err).To(BeNil())
}

// RestoreCD reattaches the previously mounted iso to the VM
func (s *SUT) RestoreCD() {
	By("Restoring the CD")
	out, err := exec.Command("bash", "-c", fmt.Sprintf("VBoxManage storageattach 'test' --storagectl 'sata controller' --port 1 --device 0 --type dvddrive --medium %s --forceunmount", s.CDLocation)).CombinedOutput()
	fmt.Print(string(out))
	Expect(err).To(BeNil())
}

func bash(s string) (string, error) {
	o, err := exec.Command("bash", "-c", s).CombinedOutput()
	return string(o), err
}

func (s *SUT) PowerOff() {
	_, _ = bash(fmt.Sprintf(`VBoxManage controlvm "%s" poweroff`, s.MachineID))
}

func (s *SUT) Start() {
	_, _ = bash(fmt.Sprintf(`VBoxManage startvm "%s" --type headless`, s.MachineID))
}

func (s *SUT) Snapshot() error {
	out, err := bash(fmt.Sprintf(`VBoxManage snapshot "%s" take snap`, s.MachineID))
	fmt.Println(out)
	return err
}

func (s *SUT) RestoreSnapshot() error {
	s.PowerOff()
	out, err := bash(fmt.Sprintf(`VBoxManage snapshot "%s" restore snap`, s.MachineID))
	fmt.Println(out)
	s.Start()
	return err
}

func (s SUT) GetDiskLayout(disk string) DiskLayout {
	// -b size in bytes
	// -n no headings
	// -J json output
	diskLayout := DiskLayout{}
	out, err := s.Command(fmt.Sprintf("lsblk %s -o LABEL,SIZE,FSTYPE -b -n -J", disk))
	Expect(err).To(BeNil())
	err = json.Unmarshal([]byte(strings.TrimSpace(out)), &diskLayout)
	Expect(err).To(BeNil())
	return diskLayout
}

// ElementalCmd will run the default elemental binary with some default flags useful for testing and the given args
// it allows overriding the default args just in case
func (s SUT) ElementalCmd(args ...string) string {
	eleCommand := "elemental"
	// Allow overriding the default args
	if os.Getenv("ELEMENTAL_CMD_ARGS") == "" {
		eleCommand = strings.Join([]string{eleCommand, "--debug", "--logfile", "/tmp/elemental.log"}, " ")
	} else {
		eleCommand = strings.Join([]string{eleCommand, os.Getenv("ELEMENTAL_CMD_ARGS")}, " ")
	}
	for _, arg := range args {
		eleCommand = strings.Join([]string{eleCommand, arg}, " ")
	}
	return eleCommand
}

// AssertBootedFrom asserts that we booted from the proper type and adds a helpful message
func (s SUT) AssertBootedFrom(b string) {
	ExpectWithOffset(1, s.BootFrom()).To(Equal(b), "Should have booted from: %s", b)
}
