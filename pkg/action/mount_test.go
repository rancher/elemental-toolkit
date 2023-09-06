package action_test

import (
	"bytes"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-toolkit/pkg/action"
	"github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

var _ = Describe("Mount Action", func() {
	var cfg *v1.RunConfig
	var mounter *mount.FakeMounter
	var fs vfs.FS
	var logger v1.Logger
	var cleanup func()
	var memLog *bytes.Buffer

	BeforeEach(func() {
		mounter = &mount.FakeMounter{}
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		logger.SetLevel(logrus.DebugLevel)
		fs, cleanup, _ = vfst.NewTestFS(map[string]interface{}{})
		cfg = config.NewRunConfig(
			config.WithFs(fs),
			config.WithMounter(mounter),
			config.WithLogger(logger),
		)
	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Write fstab", Label("mount", "fstab"), func() {
		It("Writes a simple fstab", func() {
			spec := &v1.MountSpec{
				WriteFstab: true,
				Image: &v1.Image{
					LoopDevice: "/dev/loop0",
				},
			}
			utils.MkdirAll(fs, filepath.Join(spec.Sysroot, "/etc"), constants.DirPerm)
			err := action.WriteFstab(cfg, spec)
			Expect(err).To(BeNil())

			fstab, err := cfg.Config.Fs.ReadFile(filepath.Join(spec.Sysroot, "/etc/fstab"))
			Expect(err).To(BeNil())
			Expect(string(fstab)).To(Equal("/dev/loop0\t/\tauto\tro\t0 0\n"))
		})
	})
	// Describe("Mount image", Label("mount", "image"), func() {
	// 	It("Mounts an image", func() {
	// 		spec := &v1.MountSpec{
	// 			Image: &v1.Image{},
	// 		}

	// 		err := action.RunMount(cfg, spec)
	// 		Expect(err).To(BeNil())
	// 	})
	// })
})
