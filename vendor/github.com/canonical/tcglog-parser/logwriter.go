// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"errors"
	"fmt"
	"io"

	"golang.org/x/xerrors"
)

// WriteLog writes an event log to w from the supplied events, in a
// format that can be read again by ReadLog. The first event must be
// a Specification ID event. If the Specification ID event specifies
// a crypto-agile log, the digest algorithms must all be supported by
// go and the specified sizes must all be correct. Each of the
// supplied events must contain a digest for each algorithm. For a
// non crypto-agile log, each of the supplied events must contain
// a SHA-1 digest.
//
// This function is only useful for generating reproducible data for
// use in tests.
func WriteLog(w io.Writer, events []*Event) error {
	if len(events) == 0 {
		return nil
	}

	var cryptoAgile bool
	var digestSizes []EFISpecIdEventAlgorithmSize

	switch d := events[0].Data.(type) {
	case *SpecIdEvent00:
		_ = d
	case *SpecIdEvent02:
		_ = d
	case *SpecIdEvent03:
		cryptoAgile = true
		digestSizes = d.DigestSizes
		for _, digest := range digestSizes {
			if !digest.AlgorithmId.IsValid() {
				return fmt.Errorf("unsupported algorithm %v", digest.AlgorithmId)
			}
			if digest.DigestSize != uint16(digest.AlgorithmId.Size()) {
				return fmt.Errorf("invalid size for algorithm %v", digest.AlgorithmId)
			}
		}
	default:
		return errors.New("first event must be a spec ID event")
	}

	for i, event := range events {
		if cryptoAgile && i > 0 {
			for _, digest := range digestSizes {
				if _, ok := event.Digests[digest.AlgorithmId]; !ok {
					return fmt.Errorf("event %d has missing digest for algorithm %v", i, digest.AlgorithmId)
				}
			}
			if err := event.WriteCryptoAgile(w); err != nil {
				return xerrors.Errorf("cannot write event %d: %w", i, err)
			}
		} else {
			if err := event.Write(w); err != nil {
				return xerrors.Errorf("cannot write event %d: %w", i, err)
			}
		}
	}

	return nil
}
