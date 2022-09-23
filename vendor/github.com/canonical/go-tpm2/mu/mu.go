// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package mu

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"runtime"
	"strings"

	"golang.org/x/xerrors"
)

var (
	customMuType   reflect.Type = reflect.TypeOf((*customMuIface)(nil)).Elem()
	emptyIfaceType reflect.Type = reflect.TypeOf((*interface{})(nil)).Elem()
	unionType      reflect.Type = reflect.TypeOf((*Union)(nil)).Elem()
	nilValueType   reflect.Type = reflect.TypeOf(NilUnionValue)
	rawBytesType   reflect.Type = reflect.TypeOf(RawBytes(nil))
)

// InvalidSelectorError may be returned as a wrapped error from UnmarshalFromBytes or UnmarshalFromReader when a union type indicates
// that a selector value is invalid.
type InvalidSelectorError struct {
	Selector reflect.Value
}

func (e *InvalidSelectorError) Error() string {
	return fmt.Sprintf("invalid selector value: %v", e.Selector)
}

type customMuIface interface {
	CustomMarshaller
	CustomUnmarshaller
}

// CustomMarshaller is implemented by types that require custom marshalling behaviour because they are non-standard and not
// directly supported by the marshalling code. This interface should be implemented by types with a value receiver if you want
// to be able to pass it directly by value to MarshalToBytes or MarshalToWriter. Implementations must also implement the
// CustomUnmarshaller interface.
//
// If the custom implementation makes a recursive call in the MarshalToWriter, it should propagate errors from
// this without wrapping. This allows the full context of the error to be surfaced from the originating call.
type CustomMarshaller interface {
	Marshal(w io.Writer) error
}

// CustomUnmarshaller is implemented by types that require custom unmarshalling behaviour because they are non-standard and not
// directly supported by the marshalling code. This interface must be implemented by types with a pointer receiver, and types
// must also implement the CustomMarshaller interface.
//
// If the custom implementation makes a recursive call in the UnmarshalFromReader, it should propagate errors from
// this without wrapping. This allows the full context of the error to be surfaced from the originating call.
type CustomUnmarshaller interface {
	Unmarshal(r Reader) error
}

type empty struct{}

// NilUnionValue is a special value, the type of which should be returned from implementations of Union.Select to indicate
// that a union contains no data for a particular selector value.
var NilUnionValue empty

// RawBytes is a special byte slice type which is marshalled and unmarshalled without a size field. The slice must be pre-allocated to
// the correct length by the caller during unmarshalling.
type RawBytes []byte

func Raw(val interface{}) interface{} {
	return &struct {
		Value interface{} `tpm2:"raw"`
	}{val}
}

// Sized converts the supplied value to a sized value.
//
// To marshal a sized value, the supplied value must be a pointer to the actual
// value.
//
// To unmarshal a sized value, the supplied value must be a pointer to the
// destionation pointer that will point to the unmarshalled value.
func Sized(val interface{}) interface{} {
	return &struct {
		Value interface{} `tpm2:"sized"`
	}{val}
}

// Union is implemented by structure types that correspond to TPMU prefixed TPM types.
type Union interface {
	// Select is called by the marshalling code to map the supplied selector to a field. The returned value must be a pointer to
	// the field to be marshalled or unmarshalled. To work correctly during marshalling and unmarshalling, implementations must
	// take a pointer receiver. If no data should be marshalled or unmarshalled, it should return NilUnionValue.
	Select(selector reflect.Value) interface{}
}

type containerNode struct {
	value reflect.Value
	index int
	entry [1]uintptr
}

type containerStack []containerNode

func (s containerStack) push(node containerNode) containerStack {
	return append(s, node)
}

func (s containerStack) pop() containerStack {
	return s[:len(s)-1]
}

func (s containerStack) top() containerNode {
	return s[len(s)-1]
}

func (s containerStack) String() string {
	str := new(bytes.Buffer)
	str.WriteString("=== BEGIN STACK ===\n")
	for i := len(s) - 1; i >= 0; i-- {
		switch {
		case reflect.PtrTo(s[i].value.Type()).Implements(customMuType):
			frames := runtime.CallersFrames(s[i].entry[:])
			frame, _ := frames.Next()
			fmt.Fprintf(str, "... %s custom type, call from %s:%d argument %d\n", s[i].value.Type(), frame.File, frame.Line, s[i].index)
		case s[i].value.Kind() == reflect.Struct:
			fmt.Fprintf(str, "... %s field %s\n", s[i].value.Type(), s[i].value.Type().Field(s[i].index).Name)
		case s[i].value.Kind() == reflect.Slice:
			fmt.Fprintf(str, "... %s index %d\n", s[i].value.Type(), s[i].index)
		default:
			panic("unsupported kind")
		}
	}
	str.WriteString("=== END STACK ===\n")

	return str.String()
}

// Error is returned from any function in this package to provide context
// of where an error occurred.
type Error struct {
	// Index indicates the argument on which this error occurred.
	Index int

	Op string

	total    int
	entry    [1]uintptr
	stack    containerStack
	leafType reflect.Type
	err      error
}

func (e *Error) Error() string {
	s := new(bytes.Buffer)
	fmt.Fprintf(s, "cannot %s argument ", e.Op)
	if e.total > 1 {
		fmt.Fprintf(s, "%d ", e.Index)
	}
	fmt.Fprintf(s, "whilst processing element of type %s: %v", e.leafType, e.err)
	if len(e.stack) != 0 {
		fmt.Fprintf(s, "\n\n%s", e.stack)
	}
	return s.String()
}

func (e *Error) Unwrap() error {
	return e.err
}

// Type returns the type of the value on which this error occurred.
func (e *Error) Type() reflect.Type {
	return e.leafType
}

// Depth returns the depth of the value on which this error occurred.
func (e *Error) Depth() int {
	return len(e.stack)
}

// Container returns the type of the container at the specified depth.
//
// If the returned type is a structure, the returned index corresponds
// to the index of the field in that structure.
//
// If the returned type is a slice, the returned index corresponds to
// the index in that slice.
//
// If the returned type implements the CustomMarshaller and
// CustomUnmarshaller interfaces, the returned index corresponds to
// the argument index in the recursive call in to one of the marshalling
// or unmarshalling APIs. The returned frame indicates where this
// recursive call originated from.
func (e *Error) Container(depth int) (containerType reflect.Type, index int, entry runtime.Frame) {
	var frame runtime.Frame
	if reflect.PtrTo(e.stack[depth].value.Type()).Implements(customMuType) {
		frames := runtime.CallersFrames(e.stack[depth].entry[:])
		frame, _ = frames.Next()
	}

	return e.stack[depth].value.Type(), e.stack[depth].index, frame
}

func newError(value reflect.Value, c *context, err error) error {
	if err == io.EOF {
		// All io.EOF is unexpected
		err = io.ErrUnexpectedEOF
	}
	muErr, isMuErr := err.(*Error)

	stack := make(containerStack, len(c.stack))
	copy(stack, c.stack)

	var leafType reflect.Type
	if isMuErr {
		// This is an error returned from a custom type.
		// Preserve the original error
		err = muErr.err

		// Copy the leaf type to the new error
		leafType = muErr.leafType

		// Append the original error stack to the new error.
		stack = stack.push(containerNode{value: value, index: muErr.Index, entry: muErr.entry})
		stack = append(stack, muErr.stack...)
	} else {
		leafType = value.Type()
	}

	return &Error{
		Index:    c.index,
		Op:       c.mode,
		total:    c.total,
		entry:    c.caller,
		stack:    stack,
		leafType: leafType,
		err:      err}
}

type options struct {
	selector string
	sized    bool
	raw      bool
}

func parseStructFieldMuOptions(f reflect.StructField) (out options) {
	s := f.Tag.Get("tpm2")
	for _, part := range strings.Split(s, ",") {
		switch {
		case strings.HasPrefix(part, "selector:"):
			out.selector = part[9:]
		case part == "sized":
			out.sized = true
		case part == "raw":
			out.raw = true
		}
	}
	return
}

type context struct {
	caller  [1]uintptr
	mode    string
	index   int
	total   int
	stack   containerStack
	options options
}

func (c *context) enterStructField(s reflect.Value, i int) (f reflect.Value, exit func()) {
	origOptions := c.options
	c.options = parseStructFieldMuOptions(s.Type().Field(i))
	c.stack = c.stack.push(containerNode{value: s, index: i})

	return s.Field(i), func() {
		c.stack = c.stack.pop()
		c.options = origOptions
	}
}

func (c *context) enterListElem(l reflect.Value, i int) (elem reflect.Value, exit func()) {
	origOptions := c.options
	c.options = options{}
	c.stack = c.stack.push(containerNode{value: l, index: i})

	return l.Index(i), func() {
		c.stack = c.stack.pop()
		c.options = origOptions
	}
}

func (c *context) enterUnionElem(u reflect.Value) (elem reflect.Value, exit func(), err error) {
	if len(c.stack) == 0 {
		panic(fmt.Sprintf("union type %s is not inside a container", u.Type()))
	}

	var selectorVal reflect.Value
	if c.options.selector == "" {
		selectorVal = c.stack.top().value.Field(0)
	} else {
		selectorVal = c.stack.top().value.FieldByName(c.options.selector)
		if !selectorVal.IsValid() {
			panic(fmt.Sprintf("selector name %s for union type %s does not reference a valid field\n%s",
				c.options.selector, u.Type(), c.stack))
		}
	}

	p := u.Addr().Interface().(Union).Select(selectorVal)
	switch {
	case p == nil:
		return reflect.Value{}, nil, &InvalidSelectorError{selectorVal}
	case p == NilUnionValue:
		return reflect.Value{}, nil, nil
	}
	pv := reflect.ValueOf(p)

	index := -1
	for i := 0; i < u.NumField(); i++ {
		if u.Field(i).Addr().Interface() == pv.Interface() {
			index = i
			break
		}
	}
	if index == -1 {
		panic(fmt.Sprintf("Union.Select implementation for type %s returned a non-member pointer\n%s",
			u.Type(), c.stack))
	}

	origOptions := c.options
	c.options.selector = ""
	c.stack = c.stack.push(containerNode{value: u, index: index})

	return pv.Elem(), func() {
		c.stack = c.stack.pop()
		c.options = origOptions
	}, nil

}

func (c *context) enterSizedType(v reflect.Value) (exit func()) {
	switch {
	case v.Kind() == reflect.Ptr:
	case v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8:
	default:
		panic(fmt.Sprintf("invalid sized type: %v", v.Type()))
	}

	origOptions := c.options
	c.options.sized = false
	if v.Kind() == reflect.Slice {
		c.options.raw = true
	}

	return func() {
		c.options = origOptions
	}
}

// TPMKind indicates the TPM type class associated with a Go type
type TPMKind int

const (
	// TPMKindUnsupported indicates that a go type has no corresponding
	// TPM type class.
	TPMKindUnsupported TPMKind = iota

	// TPMKindPrimitive indicates that a go type corresponds to one
	// of the primitive TPM types (UINT8, BYTE, INT8, BOOL, UINT16,
	// INT16, UINT32, INT32, UINT64, INT64, TPM_ALG_ID, any TPMA_
	// prefixed type).
	TPMKindPrimitive

	// TPMKindSized indicates that a go type corresponds to a
	// TPM2B prefixed TPM type.
	TPMKindSized

	// TPMKindList indicates that a go type corresponds to a
	// TPML prefixed TPM type.
	TPMKindList

	// TPMKindStruct indicates that a go type corresponds to a
	// TPMS or TPMT prefixed TPM type - this package doesn't
	// distinguish between the two.
	TPMKindStruct

	// TPMKindUnion indicates that a go type corresponds to a
	// TPMU prefixed TPM type.
	TPMKindUnion

	// TPMKindCustom correponds to a go type that defines its own
	// marshalling behaviour.
	TPMKindCustom

	// TPMKindRaw corresponds to a go slice that is marshalled
	// without a size field (it is not TPMKindSized or TPMKindList).
	TPMKindRaw
)

func tpmKind(t reflect.Type, opts *options) TPMKind {
	if opts.sized {
		return TPMKindSized
	}

	if t == emptyIfaceType {
		t = t.Elem()
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if reflect.PtrTo(t).Implements(customMuType) {
		return TPMKindCustom
	}

	switch t.Kind() {
	case reflect.Bool, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TPMKindPrimitive
	case reflect.Slice:
		switch {
		case t == rawBytesType:
			return TPMKindRaw
		case opts.raw:
			return TPMKindRaw
		case t.Elem().Kind() == reflect.Uint8:
			return TPMKindSized
		default:
			return TPMKindList
		}
	case reflect.Struct:
		if reflect.PtrTo(t).Implements(unionType) {
			return TPMKindUnion
		}
		return TPMKindStruct
	default:
		return TPMKindUnsupported
	}
}

// DetermineTPMKind returns the TPMKind associated with the supplied go value. It will automatically dereference pointer types.
// Single field structures will be unwrapped and the TPMKind associated with the structure field will be returned.
func DetermineTPMKind(i interface{}) TPMKind {
	t := reflect.TypeOf(i)
	var opts options

	for {
		k := tpmKind(t, &opts)
		if k != TPMKindStruct {
			return k
		}

		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		if t.NumField() != 1 {
			return k
		}
		f := t.Field(0)
		opts = parseStructFieldMuOptions(f)
		t = f.Type
	}
}

type marshaller struct {
	*context
	w      io.Writer
	nbytes int
}

func (m *marshaller) Write(p []byte) (n int, err error) {
	n, err = m.w.Write(p)
	m.nbytes += n
	return
}

func (m *marshaller) marshalSized(v reflect.Value) error {
	exit := m.enterSizedType(v)
	defer exit()

	if v.IsNil() {
		if err := binary.Write(m, binary.BigEndian, uint16(0)); err != nil {
			return newError(v, m.context, err)
		}
		return nil
	}

	tmpBuf := new(bytes.Buffer)
	sm := &marshaller{context: m.context, w: tmpBuf}
	if err := sm.marshalValue(v); err != nil {
		return err
	}
	if tmpBuf.Len() > math.MaxUint16 {
		return newError(v, m.context, errors.New("sized value size greater than 2^16-1"))
	}
	if err := binary.Write(m, binary.BigEndian, uint16(tmpBuf.Len())); err != nil {
		return newError(v, m.context, err)
	}
	if _, err := tmpBuf.WriteTo(m); err != nil {
		return newError(v, m.context, err)
	}
	return nil
}

func (m *marshaller) marshalRawList(v reflect.Value) error {
	for i := 0; i < v.Len(); i++ {
		elem, exit := m.enterListElem(v, i)
		if err := m.marshalValue(elem); err != nil {
			exit()
			return err
		}
		exit()
	}
	return nil
}

func (m *marshaller) marshalRaw(v reflect.Value) error {
	switch v.Type().Elem().Kind() {
	case reflect.Uint8:
		if _, err := m.Write(v.Bytes()); err != nil {
			return newError(v, m.context, err)
		}
		return nil
	default:
		return m.marshalRawList(v)
	}
}

func (m *marshaller) marshalPtr(v reflect.Value) error {
	p := v
	if v.IsNil() {
		p = reflect.New(v.Type().Elem())
	}
	return m.marshalValue(p.Elem())
}

func (m *marshaller) marshalPrimitive(v reflect.Value) error {
	if err := binary.Write(m, binary.BigEndian, v.Interface()); err != nil {
		return newError(v, m.context, err)
	}
	return nil
}

func (m *marshaller) marshalList(v reflect.Value) error {
	// int is either 32-bits or 64-bits. We can't compare slice.Len() to math.MaxUint32 when int is 32-bits and it isn't
	// necessary anyway. For the case where int is 64-bits, truncate to uint32 then zero extend it again to int to make
	// sure the original number was preserved.
	if int(uint32(v.Len())) != v.Len() {
		return newError(v, m.context, errors.New("slice length greater than 2^32-1"))
	}

	// Marshal length field
	if err := binary.Write(m, binary.BigEndian, uint32(v.Len())); err != nil {
		return newError(v, m.context, err)
	}

	return m.marshalRawList(v)
}

func (m *marshaller) marshalStruct(v reflect.Value) error {
	for i := 0; i < v.NumField(); i++ {
		f, exit := m.enterStructField(v, i)
		if err := m.marshalValue(f); err != nil {
			exit()
			return err
		}
		exit()
	}

	return nil
}

func (m *marshaller) marshalUnion(v reflect.Value) error {
	// Ignore during marshalling - let the TPM unmarshalling catch it
	elem, exit, _ := m.enterUnionElem(v)
	if !elem.IsValid() {
		return nil
	}
	defer exit()
	return m.marshalValue(elem)
}

func (m *marshaller) marshalCustom(v reflect.Value) error {
	if err := v.Interface().(CustomMarshaller).Marshal(m); err != nil {
		return newError(v, m.context, err)
	}
	return nil
}

func (m *marshaller) marshalValue(v reflect.Value) error {
	kind := tpmKind(v.Type(), &m.options)

	if v.Type() == emptyIfaceType {
		v = v.Elem()
	}
	if v.Kind() == reflect.Ptr && kind != TPMKindSized {
		return m.marshalPtr(v)
	}

	switch kind {
	case TPMKindPrimitive:
		return m.marshalPrimitive(v)
	case TPMKindSized:
		return m.marshalSized(v)
	case TPMKindList:
		return m.marshalList(v)
	case TPMKindStruct:
		return m.marshalStruct(v)
	case TPMKindUnion:
		return m.marshalUnion(v)
	case TPMKindCustom:
		return m.marshalCustom(v)
	case TPMKindRaw:
		return m.marshalRaw(v)
	}

	panic(fmt.Sprintf("cannot marshal unsupported type %s", v.Type()))
}

func (m *marshaller) marshal(vals ...interface{}) (int, error) {
	m.total = len(vals)
	m.nbytes = 0
	for i, v := range vals {
		m.index = i
		if err := m.marshalValue(reflect.ValueOf(v)); err != nil {
			return m.nbytes, err
		}
	}
	return m.nbytes, nil
}

// Reader is an interface that groups the io.Reader interface with an additional method to
// obtain the remaining number of bytes that can be read for implementations that support this.
type Reader interface {
	io.Reader
	Len() int
}

type unmarshaller struct {
	*context
	r      io.Reader
	sz     int64
	nbytes int
}

func (u *unmarshaller) Read(p []byte) (n int, err error) {
	n, err = u.r.Read(p)
	u.nbytes += n
	return
}

func (u *unmarshaller) Len() int {
	return int(u.sz - int64(u.nbytes))
}

func startingSizeOfReader(r io.Reader) (int64, error) {
	switch rImpl := r.(type) {
	case *os.File:
		fi, err := rImpl.Stat()
		if err != nil {
			return 0, err
		}
		if fi.Mode().IsRegular() {
			start, err := rImpl.Seek(0, io.SeekCurrent)
			if err != nil {
				return 0, err
			}
			return fi.Size() - start, nil
		}
	case *bytes.Reader:
		return int64(rImpl.Len()), nil
	case *bytes.Buffer:
		return int64(rImpl.Len()), nil
	case *io.SectionReader:
		start, _ := rImpl.Seek(0, io.SeekCurrent)
		return rImpl.Size() - start, nil
	case *io.LimitedReader:
		sz, err := startingSizeOfReader(rImpl.R)
		if err != nil {
			return 0, err
		}
		if rImpl.N < sz {
			sz = rImpl.N
		}
		return sz, nil
	}
	return 1<<63 - 1, nil
}

func makeUnmarshaller(ctx *context, r io.Reader) (*unmarshaller, error) {
	sz, err := startingSizeOfReader(r)
	if err != nil {
		return nil, err
	}
	return &unmarshaller{context: ctx, r: r, sz: sz}, nil
}

func (u *unmarshaller) unmarshalSized(v reflect.Value) error {
	exit := u.enterSizedType(v)
	defer exit()

	var size uint16
	if err := binary.Read(u, binary.BigEndian, &size); err != nil {
		return newError(v, u.context, err)
	}

	switch {
	case size == 0 && !v.IsNil() && v.Kind() == reflect.Ptr:
		return newError(v, u.context, errors.New("sized value is zero sized, but destination value has been pre-allocated"))
	case size == 0:
		return nil
	case int(size) > u.Len():
		return newError(v, u.context, errors.New("sized value has a size larger than the remaining bytes"))
	case v.Kind() == reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), int(size), int(size)))
	}

	su, err := makeUnmarshaller(u.context, io.LimitReader(u, int64(size)))
	if err != nil {
		return newError(v, u.context, xerrors.Errorf("cannot create new reader for sized payload: %w", err))
	}
	return su.unmarshalValue(v)
}

func (u *unmarshaller) unmarshalRawList(v reflect.Value, n int) (reflect.Value, error) {
	for i := 0; i < n; i++ {
		v = reflect.Append(v, reflect.Zero(v.Type().Elem()))
		elem, exit := u.enterListElem(v, i)
		if err := u.unmarshalValue(elem); err != nil {
			exit()
			return reflect.Value{}, err
		}
		exit()
	}
	return v, nil
}

func (u *unmarshaller) unmarshalRaw(v reflect.Value) error {
	switch v.Type().Elem().Kind() {
	case reflect.Uint8:
		if _, err := io.ReadFull(u, v.Bytes()); err != nil {
			return newError(v, u.context, err)
		}
		return nil
	default:
		_, err := u.unmarshalRawList(v.Slice(0, 0), v.Len())
		return err
	}
}

func (u *unmarshaller) unmarshalPtr(v reflect.Value) error {
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	return u.unmarshalValue(v.Elem())
}

func (u *unmarshaller) unmarshalPrimitive(v reflect.Value) error {
	if err := binary.Read(u, binary.BigEndian, v.Addr().Interface()); err != nil {
		return newError(v, u.context, err)
	}
	return nil
}

func (u *unmarshaller) unmarshalList(v reflect.Value) error {
	// Unmarshal the length
	var length uint32
	if err := binary.Read(u, binary.BigEndian, &length); err != nil {
		return newError(v, u.context, err)
	}

	if v.IsNil() || v.Cap() < int(length) {
		v.Set(reflect.MakeSlice(v.Type(), 0, int(length)))
	}

	s, err := u.unmarshalRawList(v.Slice(0, 0), int(length))
	if err != nil {
		return err
	}
	v.Set(s)
	return nil
}

func (u *unmarshaller) unmarshalStruct(v reflect.Value) error {
	for i := 0; i < v.NumField(); i++ {
		elem, exit := u.enterStructField(v, i)
		if err := u.unmarshalValue(elem); err != nil {
			exit()
			return err
		}
		exit()
	}
	return nil
}

func (u *unmarshaller) unmarshalUnion(v reflect.Value) error {
	elem, exit, err := u.enterUnionElem(v)
	if err != nil {
		return newError(v, u.context, err)
	}
	if !elem.IsValid() {
		return nil
	}
	defer exit()
	return u.unmarshalValue(elem)
}

func (u *unmarshaller) unmarshalCustom(v reflect.Value) error {
	if err := v.Addr().Interface().(CustomUnmarshaller).Unmarshal(u); err != nil {
		return newError(v, u.context, err)
	}
	return nil
}

func (u *unmarshaller) unmarshalValue(v reflect.Value) error {
	kind := tpmKind(v.Type(), &u.options)

	if v.Type() == emptyIfaceType {
		v = v.Elem()
	}
	if v.Kind() == reflect.Ptr && kind != TPMKindSized {
		return u.unmarshalPtr(v)
	}

	switch kind {
	case TPMKindPrimitive:
		return u.unmarshalPrimitive(v)
	case TPMKindSized:
		return u.unmarshalSized(v)
	case TPMKindList:
		return u.unmarshalList(v)
	case TPMKindStruct:
		return u.unmarshalStruct(v)
	case TPMKindUnion:
		return u.unmarshalUnion(v)
	case TPMKindCustom:
		return u.unmarshalCustom(v)
	case TPMKindRaw:
		return u.unmarshalRaw(v)
	}

	panic(fmt.Sprintf("cannot unmarshal unsupported type %s", v.Type()))
}

func (u *unmarshaller) unmarshal(vals ...interface{}) (int, error) {
	u.total = len(vals)
	u.nbytes = 0
	for i, v := range vals {
		u.index = i
		if err := u.unmarshalValue(reflect.ValueOf(v).Elem()); err != nil {
			return u.nbytes, err
		}
	}
	return u.nbytes, nil
}

func marshalToWriter(skip int, w io.Writer, vals ...interface{}) (int, error) {
	var caller [1]uintptr
	runtime.Callers(skip+1, caller[:])

	m := &marshaller{context: &context{caller: caller, mode: "marshal"}, w: w}
	return m.marshal(vals...)
}

// MarshalToWriter marshals vals to w in the TPM wire format, according to the rules specified in the package description.
//
// Pointers are automatically dereferenced. Nil pointers are marshalled to the zero value for the pointed to type, unless
// the pointer is to a sized structure (a struct field with the 'tpm2:"sized"` tag pointing to another struct), in which case
// a value of zero size is marshalled.
//
// The number of bytes written to w are returned. If this function does not complete successfully, it will return an error and
// the number of bytes written.
//
// This function only returns an error if a sized value (sized buffer, sized structure or list) is too large for its corresponding
// size field, or if the supplied io.Writer returns an error.
func MarshalToWriter(w io.Writer, vals ...interface{}) (int, error) {
	return marshalToWriter(2, w, vals...)
}

// MustMarshalToWriter is the same as MarshalToWriter, except that it panics if it encounters an error.
func MustMarshalToWriter(w io.Writer, vals ...interface{}) int {
	n, err := marshalToWriter(2, w, vals...)
	if err != nil {
		panic(err)
	}
	return n
}

func marshalToBytes(skip int, vals ...interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	if _, err := marshalToWriter(skip+1, buf, vals...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalToBytes marshals vals to TPM wire format, according to the rules specified in the package description.
//
// Pointers are automatically dereferenced. Nil pointers are marshalled to the zero value for the pointed to type, unless
// the pointer is to a sized structure (a struct field with the 'tpm2:"sized"` tag pointing to another struct), in which case
// a value of zero size is marshalled.
//
// The number of bytes written to w are returned. If this function does not complete successfully, it will return an error and
// the number of bytes written.
//
// This function only returns an error if a sized value (sized buffer, sized structure or list) is too large for its corresponding
// size field.
func MarshalToBytes(vals ...interface{}) ([]byte, error) {
	return marshalToBytes(2, vals...)
}

// MustMarshalToBytes is the same as MarshalToBytes, except that it panics if it encounters an error.
func MustMarshalToBytes(vals ...interface{}) []byte {
	b, err := marshalToBytes(2, vals...)
	if err != nil {
		panic(err)
	}
	return b
}

func unmarshalFromReader(skip int, r io.Reader, vals ...interface{}) (int, error) {
	var caller [1]uintptr
	runtime.Callers(skip+1, caller[:])

	for _, val := range vals {
		v := reflect.ValueOf(val)
		if v.Kind() != reflect.Ptr {
			panic(fmt.Sprintf("cannot unmarshal to non-pointer type %s", v.Type()))
		}

		if v.IsNil() {
			panic(fmt.Sprintf("cannot unmarshal to nil pointer of type %s", v.Type()))
		}
	}

	u, err := makeUnmarshaller(&context{caller: caller, mode: "unmarshal"}, r)
	if err != nil {
		return 0, err
	}
	return u.unmarshal(vals...)
}

// UnmarshalFromReader unmarshals data in the TPM wire format from r to vals, according to the rules specified in the package
// description. The values supplied to this function must be pointers to the destination values.
//
// Pointers are automatically dererefenced. If a pointer is nil, then memory is allocated for the values and the pointer
// is initialized accordingly, unless the pointer is to a sized structure (a struct field with the 'tpm2:"sized"' tag pointing
// to another struct) and the values being unmarshalled has a zero size, in which case the pointer is not initialized. If
// a pointer is already initialized by the caller, then this function will unmarshal to the already allocated memory.
//
// Slices are allocated automatically, unless the caller has already allocated a slice that has a large enough capacity
// to hold the unmarshalled values, in which case the already allocated slice will be used and its length set accordingly.
//
// This can unmarshal raw slices (those without a corresponding size or length fields, represented by the RawBytes type or
// a slice value referenced from a struct field with the 'tpm2:"raw"' tag), but the caller must pre-allocate a slice of the
// correct size first. This function cannot allocate a slice because it doesn't have a way to determine the size to allocate.
//
// The number of bytes read from r are returned. If this function does not complete successfully, it will return an error and
// the number of bytes read. In this case, partial results may have been unmarshalled to the supplied destination values.
func UnmarshalFromReader(r io.Reader, vals ...interface{}) (int, error) {
	return unmarshalFromReader(2, r, vals...)
}

// UnmarshalFromReader unmarshals data in the TPM wire format from b to vals, according to the rules specified in the package
// description. The values supplied to this function must be pointers to the destination values.
//
// Pointers are automatically dererefenced. If a pointer is nil, then memory is allocated for the value and the pointer
// is initialized accordingly, unless the pointer is to a sized structure (a struct field with the 'tpm2:"sized"' tag pointing
// to another struct) and the value being unmarshalled has a zero size, in which case the pointer is not initialized. If
// a pointer is already initialized by the caller, then this function will unmarshal to the already allocated memory.
//
// Slices are allocated automatically, unless the caller has already allocated a slice that has a large enough capacity
// to hold the unmarshalled values, in which case the already allocated slice will be used and its length set accordingly.
//
// This can unmarshal raw slices (those without a corresponding size or length fields, represented by the RawBytes type or
// a slice value referenced from a struct field with the 'tpm2:"raw"' tag), but the caller must pre-allocate a slice of the
// correct size first. This function cannot allocate a slice because it doesn't have a way to determine the size to allocate.
//
// The number of bytes consumed from b are returned. If this function does not complete successfully, it will return an error and
// the number of bytes consumed. In this case, partial results may have been unmarshalled to the supplied destination values.
func UnmarshalFromBytes(b []byte, vals ...interface{}) (int, error) {
	buf := bytes.NewReader(b)
	return unmarshalFromReader(2, buf, vals...)
}

func copyValue(skip int, dst, src interface{}) error {
	buf := new(bytes.Buffer)
	if _, err := marshalToWriter(skip+1, buf, src); err != nil {
		return err
	}
	_, err := unmarshalFromReader(skip+1, buf, dst)
	return err
}

// CopyValue copies the value of src to dst. The destination must be a pointer to the actual
// destination value. This works by serializing the source value in the TPM wire format
// and the deserializing it again into the destination.
//
// This will return an error for any reason that would cause MarshalToBytes or
// UnmarshalFromBytes to return an error.
func CopyValue(dst, src interface{}) error {
	return copyValue(2, dst, src)
}

// MustCopyValue is the same as CopyValue except that it panics if it encounters an error.
func MustCopyValue(dst, src interface{}) {
	if err := copyValue(2, dst, src); err != nil {
		panic(err)
	}
}
