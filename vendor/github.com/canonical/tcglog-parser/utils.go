// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"bytes"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

func makeDefaultFormatter(s fmt.State, f rune) string {
	var builder bytes.Buffer
	builder.WriteString("%%")
	for _, flag := range [...]int{'+', '-', '#', ' ', '0'} {
		if s.Flag(flag) {
			fmt.Fprintf(&builder, "%c", flag)
		}
	}
	if width, ok := s.Width(); ok {
		fmt.Fprintf(&builder, "%d", width)
	}
	if prec, ok := s.Precision(); ok {
		fmt.Fprintf(&builder, ".%d", prec)
	}
	fmt.Fprintf(&builder, "%c", f)
	return builder.String()
}

func convertStringToUtf16(str string) []uint16 {
	var unicodePoints []rune
	for len(str) > 0 {
		r, s := utf8.DecodeRuneInString(str)
		unicodePoints = append(unicodePoints, r)
		str = str[s:]
	}
	return utf16.Encode(unicodePoints)
}

func convertUtf16ToString(u []uint16) string {
	var utf8Str []byte
	for _, r := range utf16.Decode(u) {
		utf8Char := make([]byte, utf8.RuneLen(r))
		utf8.EncodeRune(utf8Char, r)
		utf8Str = append(utf8Str, utf8Char...)
	}
	return string(utf8Str)
}
