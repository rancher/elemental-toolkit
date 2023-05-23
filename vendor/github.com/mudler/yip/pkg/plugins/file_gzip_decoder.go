package plugins

import (
	"bytes"
	"compress/gzip"
	"fmt"

	"github.com/sirupsen/logrus"
)

type gzipDecoder struct{}

func (gzipDecoder) Decode(content string) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader([]byte(content)))
	if err != nil {
		return nil, fmt.Errorf("unable to decode gzip: %q", err)
	}
	defer func() {
		if err := gzr.Close(); err != nil {
			logrus.Errorf("unable to close gzip reader: %q", err)
		}
	}()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(gzr); err != nil {
		return nil, fmt.Errorf("unable to read gzip: %q", err)
	}
	return buf.Bytes(), nil
}
