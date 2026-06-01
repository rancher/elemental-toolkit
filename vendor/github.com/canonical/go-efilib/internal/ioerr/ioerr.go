// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package ioerr

import (
	"errors"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
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

// EOFIsUnexpected converts [io.EOF] errors into [io.ErrUnexpectedEOF], which is
// useful when using [binary.Read] to decode parts of a structure that aren't
// at the start and when a [io.EOF] error is not expected.
//
// It can be called in one of 2 ways:
// - With a single argument which must be one of:
//   - error: in this case, the supplied error is returned untouched unless it is
//     [io.EOF], in which case, it will be returned as [io.ErrUnexpectedEOF]. This
//     only works on unwrapped [io.EOF] errors.
//   - nil: in this case, a nil error is returned.
//   - With multiple arguments - the first one must be a format string and the rest
//     being an arbitrary number of arguments. This is converted to an error using
//     [fmt.Errorf], with any [io.EOF] arguments converted to [io.ErrUnexpectedEOF].
//
// This will panic if a single argument is supplied which isn't an error or nil.
// It will also panic if multiple arguments is supplied and the first argument is
// not a format string.
func EOFIsUnexpected(args ...any) error {
	switch {
	case len(args) > 1:
		format, ok := args[0].(string)
		if !ok {
			panic(fmt.Sprintf("expected a format string, got %T", args[0]))
		}
		idx := parsePercentW(format)
		if idx >= 0 {
			if err, isErr := args[idx+1].(error); isErr && err == io.EOF {
				args[idx+1] = io.ErrUnexpectedEOF
			}
		}
		return fmt.Errorf(format, args[1:]...)
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

// PassRawEOF converts any wrapped or unwrapped [io.EOF] into a plain [io.EOF].
//
// It can be called in one of 2 ways:
// - With a single argument, which must be one of:
//   - error: In this case, if the supplied error is a wrapped or unwrapped [io.EOF],
//     a raw [io.EOF] is returned.
//   - nil: in this case, a nil error is returned.
//   - With multipple arguments - the first one must be a format string and the rest
//     being an arbitrary number of arguments. This will be converted into an error
//     using [fmt.Errorf] and that error is then passed to a nested PassRawEO.
func PassRawEOF(args ...any) error {
	switch {
	case len(args) > 1:
		format, ok := args[0].(string)
		if !ok {
			panic(fmt.Sprintf("expected a format string, got %T", args[0]))
		}
		return PassRawEOF(fmt.Errorf(format, args[1:]...))
	case len(args) == 1:
		switch err := args[0].(type) {
		case error:
			if errors.Is(err, io.EOF) {
				return io.EOF
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
