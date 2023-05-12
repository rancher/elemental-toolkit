package plugins

import (
	"net/http"
	"os"
	"time"

	"github.com/cavaliergopher/grab"
	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
)

func grabClient(timeout int) *grab.Client {
	return &grab.Client{
		UserAgent: "grab",
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
}

func Download(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	for _, dl := range s.Downloads {
		d := &dl
		realPath, err := fs.RawPath(d.Path)
		if err == nil {
			d.Path = realPath
		}
		if err := downloadFile(l, *d); err != nil {
			log.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func downloadFile(l logger.Interface, dl schema.Download) error {
	l.Debug("Downloading file ", dl.Path, dl.URL)
	client := grabClient(dl.Timeout)

	req, err := grab.NewRequest(dl.Path, dl.URL)
	if err != nil {
		return err
	}
	resp := client.Do(req)

	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

Loop:
	for {
		select {
		case <-t.C:
			l.Debugf("  transferred %v / %v bytes (%.2f%%)\n",
				resp.BytesComplete(),
				resp.Size,
				100*resp.Progress())

		case <-resp.Done:
			// download is complete
			break Loop
		}
	}

	if err := resp.Err(); err != nil {
		return err
	}

	file := resp.Filename
	err = os.Chmod(file, os.FileMode(dl.Permissions))
	if err != nil {
		return err

	}

	if dl.OwnerString != "" {
		// FIXUP: Doesn't support fs. It reads real /etc/passwd files
		uid, gid, err := utils.GetUserDataFromString(dl.OwnerString)
		if err != nil {
			return errors.Wrap(err, "Failed getting gid")
		}
		return os.Chown(dl.Path, uid, gid)
	}

	return os.Chown(file, dl.Owner, dl.Group)
}
