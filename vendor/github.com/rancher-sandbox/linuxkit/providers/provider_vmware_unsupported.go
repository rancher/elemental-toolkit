//go:build !(linux && 386) && !(linux && amd64)

/*
Copyright © 2022 - 2023 SUSE LLC

Copyright © 2015-2017 Docker, Inc.

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

package providers

// ProviderVMware implements VMware provider interface for unsupported architectures
type ProviderVMware struct{}

// NewVMware returns a new VMware Provider
func NewVMware() *ProviderVMware {
	return nil
}

// String implements provider interface
func (p *ProviderVMware) String() string {
	return "VMWARE"
}

// Probe implements provider interface
func (p *ProviderVMware) Probe() bool {
	return false
}

// Extract implements provider interface
func (p *ProviderVMware) Extract() ([]byte, error) {
	// Get vendor data, if empty do not fail
	return nil, nil
}
