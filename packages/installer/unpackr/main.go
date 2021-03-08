package main

// A sligh readaptation from genuinetools/img https://github.com/genuinetools/img/blob/54d0ca981c1260546d43961a538550eef55c87cf/pull.go

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func main() {
	if len(os.Args) != 3 {
		logrus.Error("usage: unpackr image destination")
		os.Exit(1)
	}
	image := os.Args[1]
	destination, err := filepath.Abs(os.Args[2])
	if err != nil {
		logrus.Errorf("Invalid path %s", destination)
		os.Exit(1)
	}

	logrus.Infof("Downloading %s to %s", image, destination)
	if err := downloadAndExtractDockerImage(image, destination); err != nil {
		logrus.Error(err.Error())
		os.Exit(1)
	}
}

func downloadAndExtractDockerImage(image, dest string) error {
	temp, err := ioutil.TempDir("", "unpackr")
	if err != nil {
		return err
	}

	defer os.RemoveAll(temp)
	c, err := New(temp)
	if err != nil {
		return errors.Wrapf(err, "failed creating client")
	}
	defer c.Close()

	//log.Debug("Pulling image", image)
	listedImage, err := c.Pull(image)
	if err != nil {
		return errors.Wrapf(err, "failed listing images")

	}
	//log.Debug("Pulled:", listedImage.Target.Digest)
	logrus.Infof("Size: %s", units.BytesSize(float64(listedImage.ContentSize)))
	os.RemoveAll(dest)
	return c.Unpack(image, dest)
}
