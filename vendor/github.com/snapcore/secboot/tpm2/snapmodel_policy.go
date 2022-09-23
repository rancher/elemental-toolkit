// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package tpm2

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash"
	"io"

	"github.com/canonical/go-tpm2"

	"golang.org/x/xerrors"

	"github.com/snapcore/secboot"
)

const zeroSnapSystemEpoch uint32 = 0

func computeSnapSystemEpochDigest(alg tpm2.HashAlgorithmId, epoch uint32) tpm2.Digest {
	h := alg.NewHash()
	binary.Write(h, binary.LittleEndian, epoch)
	return h.Sum(nil)
}

type snapModelHasher interface {
	io.Writer
	Complete() ([]byte, error)
	Abort()
}

type goSnapModelHasher struct {
	hash.Hash
}

func (h *goSnapModelHasher) Complete() ([]byte, error) {
	return h.Sum(nil), nil
}

func (h *goSnapModelHasher) Abort() {}

type tpmSnapModelHasher struct {
	tpm *Connection
	seq tpm2.ResourceContext
	buf []byte
}

func (h *tpmSnapModelHasher) Write(data []byte) (int, error) {
	h.buf = append(h.buf, data...)
	return len(data), nil
}

func (h *tpmSnapModelHasher) Complete() ([]byte, error) {
	digest, _, err := h.tpm.SequenceExecute(h.seq, h.buf, tpm2.HandleNull, h.tpm.HmacSession())
	return digest, err
}

func (h *tpmSnapModelHasher) Abort() {
	// This is flushed automatically by the TPM on a successful
	// TPM2_SequenceComplete, we just need a manual flush if
	// the sequence fails.
	h.tpm.FlushContext(h.seq)
}

func computeSnapModelDigest(newHash func() (snapModelHasher, error), model secboot.SnapModel) (digest tpm2.Digest, err error) {
	signKeyId, err := base64.RawURLEncoding.DecodeString(model.SignKeyID())
	if err != nil {
		return nil, xerrors.Errorf("cannot decode signing key ID: %w", err)
	}

	h, err := newHash()
	if err != nil {
		return nil, err
	}
	defer func() { h.Abort() }()

	binary.Write(h, binary.LittleEndian, uint16(tpm2.HashAlgorithmSHA384))
	h.Write(signKeyId)
	h.Write([]byte(model.BrandID()))
	digest, err = h.Complete()
	if err != nil {
		return nil, err
	}

	h, err = newHash()
	h.Write(digest)
	h.Write([]byte(model.Model()))
	digest, err = h.Complete()
	if err != nil {
		return nil, err
	}

	h, err = newHash()
	h.Write(digest)
	h.Write([]byte(model.Series()))
	binary.Write(h, binary.LittleEndian, model.Grade().Code())
	return h.Complete()
}

// SnapModelProfileParams provides the parameters to AddSnapModelProfile.
type SnapModelProfileParams struct {
	// PCRAlgorithm is the algorithm for which to compute PCR digests for. TPMs compliant with the "TCG PC Client Platform TPM Profile
	// (PTP) Specification" Level 00, Revision 01.03 v22, May 22 2017 are required to support tpm2.HashAlgorithmSHA1 and
	// tpm2.HashAlgorithmSHA256. Support for other digest algorithms is optional.
	PCRAlgorithm tpm2.HashAlgorithmId

	// PCRIndex is the PCR that snap-bootstrap measures the model to.
	PCRIndex int

	// Models is the set of models to add to the PCR profile.
	Models []secboot.SnapModel
}

// AddSnapModelProfile adds the snap model profile to the PCR protection profile, as measured by snap-bootstrap, in order to generate
// a PCR policy that is bound to a specific set of device models. It is the responsibility of snap-bootstrap to verify the integrity
// of the model that it has measured.
//
// The profile consists of 2 measurements:
//  digestEpoch
//  digestModel
//
// digestEpoch is currently hardcoded as (where H is the digest algorithm supplied via params.PCRAlgorithm):
//  digestEpoch = H(uint32(0))
//
// A future version of this package may allow another epoch to be supplied.
//
// digestModel is computed as follows (where H is the digest algorithm supplied via params.PCRAlgorithm):
//  digest1 = H(tpm2.HashAlgorithmSHA384 || sign-key-sha3-384 || brand-id)
//  digest2 = H(digest1 || model)
//  digestModel = H(digest2 || series || grade)
// The signing key digest algorithm is encoded in little-endian format, and the sign-key-sha3-384 field is hashed in decoded (binary)
// form. The brand-id, model and series fields are hashed without null terminators. The grade field is encoded as the 32 bits from
// asserts.ModelGrade.Code in little-endian format.
//
// Separate extend operations are used because brand-id, model and series are variable length.
//
// The PCR index that snap-bootstrap measures the model to can be specified via the PCRIndex field of params.
//
// The set of models to add to the PCRProtectionProfile is specified via the Models field of params.
func AddSnapModelProfile(profile *PCRProtectionProfile, params *SnapModelProfileParams) error {
	if params.PCRIndex < 0 {
		return errors.New("invalid PCR index")
	}
	if len(params.Models) == 0 {
		return errors.New("no models provided")
	}

	profile.ExtendPCR(params.PCRAlgorithm, params.PCRIndex, computeSnapSystemEpochDigest(params.PCRAlgorithm, zeroSnapSystemEpoch))

	var subProfiles []*PCRProtectionProfile
	for _, model := range params.Models {
		if model == nil {
			return errors.New("nil model")
		}

		digest, err := computeSnapModelDigest(func() (snapModelHasher, error) {
			return &goSnapModelHasher{params.PCRAlgorithm.NewHash()}, nil
		}, model)
		if err != nil {
			return err
		}
		subProfiles = append(subProfiles, NewPCRProtectionProfile().ExtendPCR(params.PCRAlgorithm, params.PCRIndex, digest))
	}

	profile.AddProfileOR(subProfiles...)
	return nil
}

// MeasureSnapSystemEpochToTPM measures a digest of uint32(0) to the specified PCR for all
// supported PCR banks. See the documentation for AddSnapModelProfile for more details.
func MeasureSnapSystemEpochToTPM(tpm *Connection, pcrIndex int) error {
	seq, err := tpm.HashSequenceStart(nil, tpm2.HashAlgorithmNull)
	if err != nil {
		return xerrors.Errorf("cannot begin event sequence: %w", err)
	}

	var epoch [4]byte
	// This doesn't do anything whilst the epoch is zero, but keep it here
	// in case it is ever bumped.
	binary.LittleEndian.PutUint32(epoch[:], zeroSnapSystemEpoch)

	if _, err := tpm.EventSequenceExecute(tpm.PCRHandleContext(pcrIndex), seq, epoch[:], tpm.HmacSession(), nil); err != nil {
		return xerrors.Errorf("cannot execute event sequence: %w", err)
	}

	return nil
}

// MeasureSnapModelToTPM measures a digest of the supplied model assertion to the specified PCR
// for all supported PCR banks. See the documentation for AddSnapModelProfile for details of
// how the digest of the model is computed.
func MeasureSnapModelToTPM(tpm *Connection, pcrIndex int, model secboot.SnapModel) error {
	pcrSelection, err := tpm.GetCapabilityPCRs(tpm.HmacSession().IncludeAttrs(tpm2.AttrAudit))
	if err != nil {
		return xerrors.Errorf("cannot determine supported PCR banks: %w", err)
	}

	var digests tpm2.TaggedHashList
	for _, s := range pcrSelection {
		digest, err := computeSnapModelDigest(func() (snapModelHasher, error) {
			seq, err := tpm.HashSequenceStart(nil, s.Hash)
			if err != nil {
				return nil, err
			}
			return &tpmSnapModelHasher{tpm: tpm, seq: seq}, nil
		}, model)
		if err != nil {
			return xerrors.Errorf("cannot compute digest for algorithm %v: %w", s.Hash, err)
		}

		digests = append(digests, tpm2.TaggedHash{HashAlg: s.Hash, Digest: digest})
	}

	return tpm.PCRExtend(tpm.PCRHandleContext(pcrIndex), digests, tpm.HmacSession())
}
