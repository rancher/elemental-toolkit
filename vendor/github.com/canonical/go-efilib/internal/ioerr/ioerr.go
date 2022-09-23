// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package ioerr

import (
	"io"
	"unicode"
	"unicode/utf8"

	"golang.org/x/xerrors"
)

// Return the index of the first %w in format, or -1 if none.
// TODO: handle "%[N]w".
func parsePercentW(format string) int {
	// Loosely copied from golang.org/x/xerrors/fmt.go.
	n := 0
	sz := 0
	var isW bool
	for i := 0; i < len(format); i += sz {
		if format[i] != '%' {
			sz = 1
			continue
		}
		// "%%" is not a format directive.
		if i+1 < len(format) && format[i+1] == '%' {
			sz = 2
			continue
		}
		sz, isW = parsePrintfVerb(format[i:])
		if isW {
			return n
		}
		n++
	}
	return -1
}

// Parse the printf verb starting with a % at s[0].
// Return how many bytes it occupies and whether the verb is 'w'.
func parsePrintfVerb(s string) (int, bool) {
	// Assume only that the directive is a sequence of non-letters followed by a single letter.
	sz := 0
	var r rune
	for i := 1; i < len(s); i += sz {
		r, sz = utf8.DecodeRuneInString(s[i:])
		if unicode.IsLetter(r) {
			return i + sz, r == 'w'
		}
	}
	return len(s), false
}

// EOFIsUnexpected converts io.EOF errors into io.ErrUnexpected, which is
// useful when using binary.Read to decode aprts of a structure that aren't
// at the start and when a io.EOF error is not expected.
//
// It can be called in one of 2 ways - either with a single argument which
// must be an error, or with a format string and an arbitrary number of
// arguments. In this second mode, the function is a wrapper around
// xerrors.Errorf.
//
// This only works on raw io.EOF errors - ie, it won't work on errors that
// have been wrapped.
func EOFIsUnexpected(args ...interface{}) error {
	switch {
	case len(args) > 1:
		format := args[0].(string)
		idx := parsePercentW(format)
		if idx >= 0 {
			if err, isErr := args[idx+1].(error); isErr && err == io.EOF {
				args[idx+1] = io.ErrUnexpectedEOF
			}
		}
		return xerrors.Errorf(format, args[1:]...)
	case len(args) == 1:
		switch err := args[0].(type) {
		case error:
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		case nil:
			return nil
		default:
			panic("invalid type")
		}
	default:
		panic("no arguments")
	}
}

// PassRawEOF is a wrapper around xerrors.Errorf that will return a raw
// io.EOF if this is the error.
func PassRawEOF(format string, args ...interface{}) error {
	err := xerrors.Errorf(format, args...)
	if xerrors.Is(err, io.EOF) {
		return io.EOF
	}
	return err
}
