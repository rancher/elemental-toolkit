//go:build !go1.20
// +build !go1.20

// Copyright 2025 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

func formatString(state fmt.State, verb rune) string {
	// 1 byte for "%", 5 bytes for flags, 7 bytes for max width, 1 period, 7 bytes for max precision, 1 byte for verb.
	// Go's fmt package caps width and precision to 1e6.
	var tmp [1 + 5 + 7 + 1 + 7 + 1]byte

	b := append(tmp[:0], '%')
	for _, c := range []byte{'+', '-', '#', ' ', '0'} {
		if !state.Flag(int(c)) {
			continue
		}
		b = append(b, c)
	}
	if w, ok := state.Width(); ok {
		b = strconv.AppendInt(b, int64(w), 10)
	}
	if p, ok := state.Precision(); ok {
		b = append(b, '.')
		b = strconv.AppendInt(b, int64(p), 10)
	}
	b = utf8.AppendRune(b, verb)
	return string(b)
}
