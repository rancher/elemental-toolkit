package pkcs7

import (
	"bytes"
	"errors"
	"math"

	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"
)

type asn1Element interface {
	Add(builder *cryptobyte.Builder)
}

type asn1Primitive struct {
	tag      cryptobyte_asn1.Tag
	contents []byte
}

func (p asn1Primitive) Add(builder *cryptobyte.Builder) {
	builder.AddASN1(p.tag, func(child *cryptobyte.Builder) {
		child.AddBytes(p.contents)
	})
}

type asn1Structured struct {
	tag      cryptobyte_asn1.Tag
	contents []asn1Element
}

func (s asn1Structured) Add(builder *cryptobyte.Builder) {
	builder.AddASN1(s.tag, func(child *cryptobyte.Builder) {
		for _, elem := range s.contents {
			elem.Add(child)
		}
	})
}

func parseBase256Int(bytes []byte) (int, error) {
	// x/crypto/cryptobyte expects these to fit into an int32, so numbers up to
	// 4 bytes are valid. If there are more bytes, make sure they're all
	// leading zeros.
	for len(bytes) > 4 {
		b := bytes[0]
		bytes = bytes[1:]
		if b != 0 {
			return 0, errors.New("base-256 number too large")
		}
	}

	var ret64 int64
	for i := 0; i < len(bytes); i++ {
		b := bytes[i]
		n := len(bytes) - i - 1
		ret64 |= int64(b) << (8 * n)
	}
	if ret64 > math.MaxInt32 {
		return 0, errors.New("base-256 number too large")
	}

	return int(ret64), nil

}

type reader struct {
	r *bytes.Reader
	n int
}

func newReader(data []byte) *reader {
	return &reader{r: bytes.NewReader(data)}
}

func (r *reader) Read(data []byte) (n int, err error) {
	n, err = r.r.Read(data)
	r.n += n
	return n, err
}

func (r *reader) ReadByte() (b byte, err error) {
	b, err = r.r.ReadByte()
	if err != nil {
		return 0, err
	}
	r.n += 1
	return b, nil
}

func readDERElement(data []byte) (int, asn1Element, error) {
	r := newReader(data)

	b, err := r.ReadByte()
	if err != nil {
		return 0, nil, errors.New("element truncated before tag")
	}
	tag := cryptobyte_asn1.Tag(b)
	if tag&0x1f == 0x1f {
		return 0, nil, errors.New("high tag numbers are not supported")
	}

	b, err = r.ReadByte()
	if err != nil {
		return 0, nil, errors.New("element truncated before length")
	}

	var length int
	switch {
	case b == 0xff:
		return 0, nil, errors.New("invalid length")
	case b > 0x80:
		bytes := make([]byte, int(b&0x7f))
		if _, err := r.Read(bytes); err != nil {
			return 0, nil, errors.New("element length base-256 truncated")
		}
		l, err := parseBase256Int(bytes)
		if err != nil {
			return 0, nil, err
		}
		length = l
	case b == 0x80:
		return 0, nil, errors.New("indefinite length elements are not supported")
	default:
		length = int(b)
	}

	content := data[r.n:]
	if length > len(content) {
		return 0, nil, errors.New("element content truncated")
	}
	content = content[:length]

	if tag&0x20 == 0 {
		return r.n + len(content), asn1Primitive{
			tag:      tag,
			contents: content}, nil
	}

	total := r.n
	ret := asn1Structured{tag: tag}

	for len(content) > 0 {
		n, elem, err := readDERElement(content)
		if err != nil {
			return total + n, nil, err
		}
		total += n
		content = content[n:]

		ret.contents = append(ret.contents, elem)
	}

	return total, ret, nil
}

// fixupDERLengths attempts to make some BER encodings compatible with go's
// encoding/asn1 package which only supports DER encoding. This does not
// convert a BER encoding in to DER, and it is not possible to do this in
// a generic way anyway because it can't handle type-specific rules for
// types with context-specific, private or application specific tags.
// What this does do is make lengths and high tag number fields properly
// DER encoded.
//
// This shouldn't be necessary because UEFI requires DER encodings, but
// there are some artefacts in the wild that have length encodings that
// aren't proper DER, such as the 2016 dbx update which contains long-form
// lengths for lengths that can be represented by the short-form encoding.
func fixupDERLengths(data []byte) ([]byte, error) {
	_, elem, err := readDERElement(data)
	if err != nil {
		return nil, err
	}

	builder := cryptobyte.NewBuilder(nil)
	elem.Add(builder)

	return builder.Bytes()

}
