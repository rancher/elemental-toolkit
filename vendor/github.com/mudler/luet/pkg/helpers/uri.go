// Copyright © 2022 Ettore Di Giacinto <mudler@gentoo.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package helpers

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/pkg/errors"
)

func IsUrl(s string) bool {
	url, err := url.Parse(s)
	if err != nil || url.Scheme == "" {
		return false
	}
	return true
}

func GetURI(s string) (string, error) {
	f, err := os.Stat(s)

	switch {
	case err == nil && f.IsDir():
		return "", errors.New("directories not supported")
	case err == nil:
		b, err := ioutil.ReadFile(s)
		return string(b), err
	case IsUrl(s):
		resp, err := http.Get(s)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		buf := bytes.NewBuffer([]byte{})
		_, err = io.Copy(buf, resp.Body)
		if err != nil {
			return "", err
		}
		return buf.String(), nil
	default:
		return "", errors.New("not supported")
	}
}
