//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package types

import "errors"

var (
	ErrNoDB = errors.New("No pci-ids DB files found (and network fetch disabled)")
	// Backwards-compat, deprecated, pleas reference ErrNoDB
	ERR_NO_DB = ErrNoDB
)
