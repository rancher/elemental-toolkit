/*
Copyright © 2021 SUSE LLC

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

// ImageSource represents the source from where an image is created for easy identification
type ImageSource struct {
	source    string
	isDir     bool
	isChannel bool
	isDocker  bool
	isFile    bool
}

func (i ImageSource) Value() string {
	return i.source
}

func (i ImageSource) IsDocker() bool {
	return i.isDocker
}

func (i ImageSource) IsChannel() bool {
	return i.isChannel
}

func (i ImageSource) IsDir() bool {
	return i.isDir
}

func (i ImageSource) IsFile() bool {
	return i.isFile
}

func NewEmptySrc() ImageSource {
	return ImageSource{}
}

func NewDockerSrc(src string) ImageSource {
	return ImageSource{source: src, isDocker: true}
}

func NewFileSrc(src string) ImageSource {
	return ImageSource{source: src, isFile: true}
}

func NewChannelSrc(src string) ImageSource {
	return ImageSource{source: src, isChannel: true}
}

func NewDirSrc(src string) ImageSource {
	return ImageSource{source: src, isDir: true}
}
