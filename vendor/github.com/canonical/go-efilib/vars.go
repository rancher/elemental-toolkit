// Copyright 2020-2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/canonical/go-efilib/internal/uefi"
)

type VariableAttributes uint32

const (
	AttributeNonVolatile                       VariableAttributes = uefi.EFI_VARIABLE_NON_VOLATILE
	AttributeBootserviceAccess                 VariableAttributes = uefi.EFI_VARIABLE_BOOTSERVICE_ACCESS
	AttributeRuntimeAccess                     VariableAttributes = uefi.EFI_VARIABLE_RUNTIME_ACCESS
	AttributeHardwareErrorRecord               VariableAttributes = uefi.EFI_VARIABLE_HARDWARE_ERROR_RECORD
	AttributeAuthenticatedWriteAccess          VariableAttributes = uefi.EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS
	AttributeTimeBasedAuthenticatedWriteAccess VariableAttributes = uefi.EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS
	AttributeAppendWrite                       VariableAttributes = uefi.EFI_VARIABLE_APPEND_WRITE
	AttributeEnhancedAuthenticatedAccess       VariableAttributes = uefi.EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS
)

var (
	ErrVarsUnavailable = errors.New("no variable backend is available")
	ErrVarNotExist     = errors.New("variable does not exist")
	ErrVarPermission   = errors.New("permission denied")
)

// VariableDescriptor represents the identity of a variable.
type VariableDescriptor struct {
	Name string
	GUID GUID
}

// VarsBackendKey is used to key a [VarsBackend] or [VarsBackend2] on a [context.Context].
type VarsBackendKey struct{}

// VarsBackend is used by the [ReadVariable], [WriteVariable] and [ListVariables]
// functions, and indirectly by other functions in this package to abstract access
// to a specific backend. A default backend is initialized at process initialization
// and is available via [DefaultVarContext].
type VarsBackend interface {
	Get(name string, guid GUID) (VariableAttributes, []byte, error)
	Set(name string, guid GUID, attrs VariableAttributes, data []byte) error
	List() ([]VariableDescriptor, error)
}

// VarsBackend2 is like [VarsBackend] only it takes a context that the backend can use
// for deadlines or cancellation - this is paricularly applicable on systems where there
// may be multiple writers and writes have to be serialized by the operating system to
// some degree.
type VarsBackend2 interface {
	Get(ctx context.Context, name string, guid GUID) (VariableAttributes, []byte, error)
	Set(ctx context.Context, name string, guid GUID, attrs VariableAttributes, data []byte) error
	List(ctx context.Context) ([]VariableDescriptor, error)
}

type varsBackendWrapper struct {
	Backend VarsBackend
}

func (v *varsBackendWrapper) Get(ctx context.Context, name string, guid GUID) (VariableAttributes, []byte, error) {
	return v.Backend.Get(name, guid)
}

func (v *varsBackendWrapper) Set(ctx context.Context, name string, guid GUID, attrs VariableAttributes, data []byte) error {
	return v.Backend.Set(name, guid, attrs, data)
}

func (v *varsBackendWrapper) List(ctx context.Context) ([]VariableDescriptor, error) {
	return v.Backend.List()
}

func getVarsBackend(ctx context.Context) VarsBackend2 {
	switch v := ctx.Value(VarsBackendKey{}).(type) {
	case VarsBackend2:
		return v
	case VarsBackend:
		return &varsBackendWrapper{Backend: v}
	case nil:
		return &varsBackendWrapper{Backend: nullVarsBackend{}}
	default:
		val := ctx.Value(VarsBackendKey{})
		panic(fmt.Sprintf("invalid variable backend type %q: %#v", reflect.TypeOf(val), val))
	}
}

type nullVarsBackend struct{}

func (v nullVarsBackend) Get(name string, guid GUID) (VariableAttributes, []byte, error) {
	return 0, nil, ErrVarsUnavailable
}

func (v nullVarsBackend) Set(name string, guid GUID, attrs VariableAttributes, data []byte) error {
	return ErrVarsUnavailable
}

func (v nullVarsBackend) List() ([]VariableDescriptor, error) {
	return nil, ErrVarsUnavailable
}

// ReadVariable returns the value and attributes of the EFI variable with the specified
// name and GUID. In general, [DefaultVarContext] should be supplied to this.
func ReadVariable(ctx context.Context, name string, guid GUID) ([]byte, VariableAttributes, error) {
	attrs, data, err := getVarsBackend(ctx).Get(ctx, name, guid)
	return data, attrs, err
}

// WriteVariable writes the supplied data value with the specified attributes to the
// EFI variable with the specified name and GUID. In general, [DefaultVarContext] should
// be supplied to this.
//
// If the variable already exists, the specified attributes must match the existing
// attributes with the exception of AttributeAppendWrite.
//
// If the variable does not exist, it will be created.
func WriteVariable(ctx context.Context, name string, guid GUID, attrs VariableAttributes, data []byte) error {
	return getVarsBackend(ctx).Set(ctx, name, guid, attrs, data)
}

// ListVariables returns a sorted list of variables that can be accessed. In
// general, [DefaultVarContext] should be supplied to this.
func ListVariables(ctx context.Context) ([]VariableDescriptor, error) {
	names, err := getVarsBackend(ctx).List(ctx)
	if err != nil {
		return nil, err
	}
	sort.Stable(variableDescriptorSlice(names))
	return names, nil
}

// variableDescriptorSlice is a slice of VariableDescriptor instances that implements
// the sort.Interface interface, so that it can be sorted.
type variableDescriptorSlice []VariableDescriptor

func (l variableDescriptorSlice) Len() int {
	return len(l)
}

func (l variableDescriptorSlice) Less(i, j int) bool {
	entryI := l[i]
	entryJ := l[j]
	// Sort by GUID first
	switch bytes.Compare(entryI.GUID[:], entryJ.GUID[:]) {
	case -1:
		// i always sorts before j
		return true
	case 0:
		// The GUIDs are identical, so sort based on name
		return entryI.Name < entryJ.Name
	case 1:
		// i always sorts after j
		return false
	default:
		panic("unexpected bytes.Compare return value")
	}
}

func (l variableDescriptorSlice) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func withVarsBackend(ctx context.Context, backend VarsBackend) context.Context {
	return context.WithValue(ctx, VarsBackendKey{}, backend)
}

func withVarsBackend2(ctx context.Context, backend VarsBackend2) context.Context {
	return context.WithValue(ctx, VarsBackendKey{}, backend)
}

func newDefaultVarContext() context.Context {
	return addDefaultVarsBackend(context.Background())
}

// DefaultVarContext should generally be passed to functions that interact with
// EFI variables in order to use the default system backend for accessing EFI
// variables. It is based on a background context.
var DefaultVarContext = newDefaultVarContext()

// WithDefaultVarsBackend adds the default system backend for accesssing EFI
// variables to an existing context.
func WithDefaultVarsBackend(ctx context.Context) context.Context {
	return addDefaultVarsBackend(ctx)
}
