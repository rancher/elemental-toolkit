// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// ConvertUTF16ToUTF8 converts the supplied UTF-16 or UCS2 string
// to a UTF-8 string. If the supplied string is NULL-terminated,
// then the NULL termination is removed from the string.
func ConvertUTF16ToUTF8(in []uint16) string {
	var u8 []byte
	for _, r := range utf16.Decode(in) {
		if r == 0 {
			break
		}
		u8Char := make([]byte, utf8.RuneLen(r))
		utf8.EncodeRune(u8Char, r)
		u8 = append(u8, u8Char...)
	}
	return string(u8)
}

// ConvertUTF8ToUTF16 converts the supplied UTF-8 string to a
// UTF-16 string.
func ConvertUTF8ToUTF16(in string) []uint16 {
	var unicodeStr []rune
	for len(in) > 0 {
		r, sz := utf8.DecodeRuneInString(in)
		unicodeStr = append(unicodeStr, r)
		in = in[sz:]
	}
	return utf16.Encode(unicodeStr)
}

// ConvertUTF8ToUCS2 converts the supplied UTF-8 string to a
// UCS2 string. Any code point outside of the Basic Multilingual
// Plane cannot be represented by UCS2 and is converted to the
// replacement character.
func ConvertUTF8ToUCS2(in string) []uint16 {
	var unicodeStr []rune
	for len(in) > 0 {
		r, sz := utf8.DecodeRuneInString(in)
		if r >= 0x10000 {
			r = unicode.ReplacementChar
		}
		unicodeStr = append(unicodeStr, r)
		in = in[sz:]
	}
	return utf16.Encode(unicodeStr)
}
