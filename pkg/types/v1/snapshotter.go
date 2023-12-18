/*
Copyright © 2022 - 2023 SUSE LLC

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
)

type Snapshotter interface {
	InitSnapshotter(rootDir string) error
	StartTransaction() (*Snapshot, error)
	CloseTransaction(snap *Snapshot) error
	CloseTransactionOnError(snap *Snapshot) error
	DeleteSnapshot(id int) error
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

func NewLoopDeviceConfig() LoopDeviceConfig {
	return LoopDeviceConfig{
		FS:   constants.LinuxFs,
		Size: constants.ImgSize,
	}
}

type snapshotterConfFactory func(defConfig interface{}, data interface{}) (interface{}, error)

func newLoopDeviceConfig(defConfig interface{}, data interface{}) (interface{}, error) {
	cfg, ok := defConfig.(LoopDeviceConfig)
	if !ok {
		cfg = NewLoopDeviceConfig()
	}
	return innerConfigDecoder[LoopDeviceConfig](cfg, data)
}

var snapshotterConfFactories = map[string]snapshotterConfFactory{}

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
		return defaultConf, fmt.Errorf("failed creating a decoder to unmarshal a loop device snapshotter: %v", err)
	}
	err = dec.Decode(confMap)
	if err != nil {
		return defaultConf, fmt.Errorf("failed to decode loopdevice configuration, invalid format: %v", err)
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

		if mData["config"] != nil {
			factory := snapshotterConfFactories[c.Type]
			conf, err := factory(c.Config, mData["config"])
			if err != nil {
				return false, fmt.Errorf("failed decoding snapshotter configuration: %v", err)
			}
			c.Config = conf
		}
	}
	return false, nil
}

func init() {
	snapshotterConfFactories[constants.LoopDeviceSnapshotterType] = newLoopDeviceConfig
}
