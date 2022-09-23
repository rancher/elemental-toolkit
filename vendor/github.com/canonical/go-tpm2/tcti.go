// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

// XXX: Note that the "TCG TSS 2.0 TPM Command Transmission Interface (TCTI) API Specification"
// defines the following callbacks:
// - transmit, which is equivalent to io.Writer.
// - receive, which is equivalent to io.Reader.
// - finalize, which is equivalent to io.Closer.
// - cancel, which we don't implement at the moment because there's not a mechanism to cancel
//   an operation in go-tpm2. Perhaps if we move the call to Read() to a separate go routine and pass
//   a context around, we could cancel using a deadline. Not today though, and the Linux character
//   device driver doesn't provide a mechanism to cancel so it probably wouldn't be worth the effort
//   anyway.
// - getPollHandles, doesn't really make sense here because go's runtime does the polling on
//   Read.
// - setLocality.
// - makeSticky, not implemented yet by any TCTI implementation in tss2 AFAICT.

// TCTI represents a communication channel to a TPM implementation.
type TCTI interface {
	// Read is used to receive a response to a previously transmitted command. The
	// implementation must support partial reading of a response, and must return io.EOF
	// when there are no more bytes of a response left to read.
	Read(p []byte) (int, error)

	// Write is used to transmit a serialized command to the TPM implementation.
	// A command must be transmitted in a single write.
	Write(p []byte) (int, error)

	Close() error

	// SetLocality sets the locality that will be used for subsequent commands.
	SetLocality(locality uint8) error

	// MakeSticky requests that the underlying resource manager does not unload the resource
	// associated with the supplied handle between commands.
	MakeSticky(handle Handle, sticky bool) error
}
