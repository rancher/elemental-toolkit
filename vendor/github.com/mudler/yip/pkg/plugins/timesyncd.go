package plugins

import (
	"os"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
	"gopkg.in/ini.v1"
)

const timeSyncd = "/etc/systemd/timesyncd.conf"

func Timesyncd(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.TimeSyncd) == 0 {
		return nil
	}
	var errs error

	path, err := fs.RawPath(timeSyncd)
	if err != nil {
		return err
	}

	if _, err := fs.Stat(timeSyncd); os.IsNotExist(err) {
		f, _ := fs.Create(timeSyncd)
		f.Close()
	}

	cfg, err := ini.Load(path)
	if err != nil {
		return err
	}

	for k, v := range s.TimeSyncd {
		cfg.Section("Time").Key(k).SetValue(v)
	}

	cfg.SaveTo(path)

	return errs
}
