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

package types

import (
	"context"
	"net/http"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/containerd/containerd/archive"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type ImageExtractor interface {
	ExtractImage(imageRef, destination, platformRef string, local bool, verify bool) (string, error)
}

type OCIImageExtractor struct{}

var _ ImageExtractor = OCIImageExtractor{}

func (e OCIImageExtractor) ExtractImage(imageRef, destination, platformRef string, local bool, verify bool) (string, error) {
	platform, err := containerregistry.ParsePlatform(platformRef)
	if err != nil {
		return "", err
	}

	opts := []name.Option{}
	if !verify {
		opts = append(opts, name.Insecure)
	}

	ref, err := name.ParseReference(imageRef, opts...)
	if err != nil {
		return "", err
	}

	var img containerregistry.Image

	err = backoff.Retry(func() error {
		img, err = image(ref, *platform, local)
		return err
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(3*time.Second), 3))
	if err != nil {
		return "", err
	}

	digest, err := img.Digest()
	if err != nil {
		return "", err
	}

	reader := mutate.Extract(img)

	_, err = archive.Apply(context.Background(), destination, reader)
	return digest.String(), err
}

func image(ref name.Reference, platform containerregistry.Platform, local bool) (containerregistry.Image, error) {
	if local {
		return daemon.Image(ref)
	}

	return remote.Image(ref,
		remote.WithTransport(http.DefaultTransport),
		remote.WithPlatform(platform),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	)
}
