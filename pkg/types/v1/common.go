/*
Copyright © 2022 - 2025 SUSE LLC

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

package v1

import (
	"fmt"
	"net/url"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/distribution/distribution/reference"
)

const (
	docker = "docker"
	oci    = "oci"
	file   = "file"
	dir    = "dir"
)

// ImageSource represents the source from where an image is created for easy identification
type ImageSource struct {
	source  string
	srcType string
}

func (i ImageSource) Value() string {
	return i.source
}

func (i ImageSource) IsImage() bool {
	return i.srcType == oci
}

func (i ImageSource) IsDir() bool {
	return i.srcType == dir
}

func (i ImageSource) IsFile() bool {
	return i.srcType == file
}

func (i ImageSource) IsEmpty() bool {
	if i.srcType == "" {
		return true
	}
	if i.source == "" {
		return true
	}
	return false
}

func (i ImageSource) String() string {
	if i.IsEmpty() {
		return ""
	}
	return fmt.Sprintf("%s://%s", i.srcType, i.source)
}

func (i ImageSource) MarshalYAML() (interface{}, error) {
	return i.String(), nil
}

func (i *ImageSource) UnmarshalYAML(value *yaml.Node) error {
	return i.updateFromURI(value.Value)
}

func (i *ImageSource) CustomUnmarshal(data interface{}) (bool, error) {
	src, ok := data.(string)
	if !ok {
		return false, fmt.Errorf("can't unmarshal %+v to an ImageSource type", data)
	}
	err := i.updateFromURI(src)
	return false, err
}

func (i *ImageSource) updateFromURI(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	scheme := u.Scheme
	value := u.Opaque
	if value == "" {
		value = filepath.Join(u.Host, u.Path)
	}
	switch scheme {
	case oci, docker:
		return i.parseImageReference(value)
	case dir:
		i.srcType = dir
		i.source = value
	case file:
		i.srcType = file
		i.source = value
	default:
		return i.parseImageReference(uri)
	}
	return nil
}

func (i *ImageSource) parseImageReference(ref string) error {
	n, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return fmt.Errorf("invalid image reference %s", ref)
	} else if reference.IsNameOnly(n) {
		ref += ":latest"
	}
	i.srcType = oci
	i.source = ref
	return nil
}

func NewSrcFromURI(uri string) (*ImageSource, error) {
	src := ImageSource{}
	err := src.updateFromURI(uri)
	return &src, err
}

func NewEmptySrc() *ImageSource {
	return &ImageSource{}
}

func NewDockerSrc(src string) *ImageSource {
	return &ImageSource{source: src, srcType: oci}
}

func NewFileSrc(src string) *ImageSource {
	return &ImageSource{source: src, srcType: file}
}

func NewDirSrc(src string) *ImageSource {
	return &ImageSource{source: src, srcType: dir}
}
