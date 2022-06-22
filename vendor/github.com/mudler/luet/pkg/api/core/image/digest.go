// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package image

import (
	"crypto/tls"
	"net/http"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Available checks if the image is available in the remote endpoint.
func Available(image string, opt ...crane.Option) bool {
	// We use crane.insecure as we just check if the image is available
	// It's the daemon duty to use it or not based on the host settings
	transport := remote.DefaultTransport.Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, //nolint: gosec
	}

	var rt http.RoundTripper = transport
	if len(opt) == 0 {
		opt = append(opt, crane.Insecure, crane.WithTransport(rt))
	}

	_, err := crane.Digest(image, opt...)
	return err == nil
}
