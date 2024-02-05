/*
Copyright © 2022 - 2024 SUSE LLC

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

	mapstructure "github.com/mitchellh/mapstructure"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	"gopkg.in/yaml.v3"
)

type Snapshotter interface {
	InitSnapshotter(rootDir string) error
	StartTransaction() (*Snapshot, error)
	CloseTransaction(snap *Snapshot) error
	CloseTransactionOnError(snap *Snapshot) error
	DeleteSnapshot(id int) error
	GetSnapshots() ([]int, error)
	SnapshotToImageSource(snap *Snapshot) (*ImageSource, error)
}

type SnapshotterConfig struct {
	Type     string      `yaml:"type,omitempty" mapstructure:"type"`
	MaxSnaps int         `yaml:"max-snaps,omitempty" mapstructure:"max-snaps"`
	Config   interface{} `yaml:"config,omitempty" mapstructure:"config"`
}

type Snapshot struct {
	ID         int
	MountPoint string
	Path       string
	WorkDir    string
	Label      string
	InProgress bool
}

type LoopDeviceConfig struct {
	Size uint   `yaml:"size,omitempty" mapstructure:"size"`
	FS   string `yaml:"fs,omitempty" mapstructure:"fs"`
}

type BtrfsConfig struct{}

func NewLoopDeviceConfig() *LoopDeviceConfig {
	return &LoopDeviceConfig{
		FS:   constants.LinuxImgFs,
		Size: constants.ImgSize,
	}
}

func NewBtrfsConfig() *BtrfsConfig {
	return &BtrfsConfig{}
}

type snapshotterConfFactory func(defConfig interface{}, data interface{}) (interface{}, error)

var snapshotterConfFactories = map[string]snapshotterConfFactory{}

func newLoopDeviceConfig(defConfig interface{}, data interface{}) (interface{}, error) {
	cfg, ok := defConfig.(*LoopDeviceConfig)
	if !ok {
		cfg = NewLoopDeviceConfig()
	}
	if data == nil {
		return cfg, nil
	}
	return innerConfigDecoder[*LoopDeviceConfig](cfg, data)
}

func newBtrfsConfig(defConfig interface{}, data interface{}) (interface{}, error) {
	cfg, ok := defConfig.(*BtrfsConfig)
	if !ok {
		cfg = NewBtrfsConfig()
	}
	if data == nil {
		return cfg, nil
	}
	return innerConfigDecoder[*BtrfsConfig](cfg, data)
}

func innerConfigDecoder[T any](defaultConf T, data interface{}) (T, error) {
	confMap, ok := data.(map[string]interface{})
	if !ok {
		return defaultConf, fmt.Errorf("invalid 'config' format for loopdevice type")
	}

	cfg := &mapstructure.DecoderConfig{
		Result: &defaultConf,
	}
	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return defaultConf, fmt.Errorf("failed creating a decoder to unmarshal a snapshotter configuration: %v", err)
	}
	err = dec.Decode(confMap)
	if err != nil {
		return defaultConf, fmt.Errorf("failed to decode snapshotter configuration, invalid format: %v", err)
	}
	return defaultConf, nil
}

func (c *SnapshotterConfig) CustomUnmarshal(data interface{}) (bool, error) {
	mData, ok := data.(map[string]interface{})
	if len(mData) > 0 && ok {
		snaphotterType, ok := mData["type"].(string)
		if ok && snaphotterType != "" {
			c.Type = snaphotterType
		} else {
			return false, fmt.Errorf("'type' is a required field for snapshotter setup")
		}

		if mData["max-snaps"] != nil {
			maxSnaps, ok := mData["max-snaps"].(int)
			if !ok {
				return false, fmt.Errorf("'max-snap' must be of integer type")
			}
			c.MaxSnaps = maxSnaps
		}

		factory := snapshotterConfFactories[c.Type]
		if factory == nil {
			return false, fmt.Errorf("failed to load snapshotter configuration for type %s", c.Type)
		}
		conf, err := factory(c.Config, mData["config"])
		if err != nil {
			return false, fmt.Errorf("failed decoding snapshotter configuration: %v", err)
		}
		c.Config = conf
	}
	return false, nil
}

func (c *SnapshotterConfig) UnmarshalYAML(node *yaml.Node) error {
	type alias SnapshotterConfig

	err := node.Decode((*alias)(c))
	if err != nil {
		return err
	}

	if c.Config != nil {
		factory := snapshotterConfFactories[c.Type]
		conf, err := factory(nil, c.Config)
		if err != nil {
			return err
		}
		c.Config = conf
	}
	return nil
}

func init() {
	snapshotterConfFactories[constants.LoopDeviceSnapshotterType] = newLoopDeviceConfig
	snapshotterConfFactories[constants.BtrfsSnapshotterType] = newBtrfsConfig
}
