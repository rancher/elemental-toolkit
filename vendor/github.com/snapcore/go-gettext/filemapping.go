package gettext

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
)

type fileMapping struct {
	data []byte

	isMapped bool
}

func (m *fileMapping) Close() error {
	runtime.SetFinalizer(m, nil)
	if !m.isMapped {
		return nil
	}
	return m.closeMapping()
}

func openMapping(f *os.File) (*fileMapping, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	m := new(fileMapping)

	if fi.Mode().IsRegular() {
		size := fi.Size()
		if size == 0 {
			return m, nil
		}
		if size < 0 {
			return nil, fmt.Errorf("file %q has negative size", fi.Name())
		}
		if size != int64(int(size)) {
			return nil, fmt.Errorf("file %q is too large", fi.Name())
		}

		if err := m.tryMap(f, size); err == nil {
			runtime.SetFinalizer(m, (*fileMapping).Close)
			return m, nil
		}
	}

	// On mapping failure, fall back to reading the file into
	// memory directly.
	m.data, err = ioutil.ReadAll(f)
	return m, err
}
