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
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"

	"golang.org/x/xerrors"
)

func isPathError(err error) bool {
	var e *os.PathError
	return xerrors.As(err, &e)
}

// isAuthFailError indicates whether the specified error is a TPM authorization check failure, with or without DA implications.
func isAuthFailError(err error, command tpm2.CommandCode, index int) bool {
	return tpm2.IsTPMSessionError(err, tpm2.ErrorAuthFail, command, index) ||
		tpm2.IsTPMSessionError(err, tpm2.ErrorBadAuth, command, index)
}

// isLoadInvalidParamError indicates whether the specified error is a TPM error associated with an invalid param
// supplied to a TPM2_Load command.
func isLoadInvalidParamError(err error) bool {
	return tpm2.IsTPMParameterError(err, tpm2.AnyErrorCode, tpm2.CommandLoad, tpm2.AnyParameterIndex)
}

func isImportInvalidParamError(err error) bool {
	// Ignore TPM_RC_SCHEME for inSymSeed, which is really an invalid parent.
	return tpm2.IsTPMParameterError(err, tpm2.AnyErrorCode, tpm2.CommandImport, tpm2.AnyParameterIndex) &&
		!tpm2.IsTPMParameterError(err, tpm2.ErrorScheme, tpm2.CommandImport, 4)
}

// isLoadInvalidParentError indicates whether the specified error is a TPM error associated with an invalid parent
// handle supplied to a TPM2_Load command.
func isLoadInvalidParentError(err error) bool {
	// TPM_RC_TYPE associated with the parent handle is an invalid parent
	return tpm2.IsTPMHandleError(err, tpm2.ErrorType, tpm2.CommandLoad, 1)
}

func isImportInvalidParentError(err error) bool {
	// TPM_RC_TYPE associated with the parent handle is an invalid parent, as is
	// TPM_RC_SCHEME associated with inSymSeed.
	return tpm2.IsTPMHandleError(err, tpm2.ErrorType, tpm2.CommandImport, 1) ||
		tpm2.IsTPMParameterError(err, tpm2.ErrorScheme, tpm2.CommandImport, 4)
}

type tpmErrorWithHandle struct {
	handle tpm2.Handle
	err    *tpm2.TPMError
}

func (e *tpmErrorWithHandle) Error() string {
	return fmt.Sprintf("%v (handle %v)", e.err, e.handle)
}

func (e *tpmErrorWithHandle) Unwrap() error {
	return e.err
}

// isTpmErrorWithHandle indicates whether the specified error is a *tpmErrorWithHandle.
func isTpmErrorWithHandle(err error) bool {
	var e *tpmErrorWithHandle
	return xerrors.As(err, &e)
}

// isObjectPrimaryKeyWithTemplate checks whether the object associated with context is primary key in the specified hierarchy with
// the specified template.
//
// This isn't completely accurate as the unique field of the template is used to seed the primary object, and this function can't
// detect if the unique field of the specified template was actually used to create the object. As a consequnce, it should be used
// with caution. This function returning true is no guarantee that recreating the object with the specified template would create
// the same object.
func isObjectPrimaryKeyWithTemplate(tpm *tpm2.TPMContext, hierarchy, object tpm2.ResourceContext, template *tpm2.Public,
	session tpm2.SessionContext) (bool, error) {
	if session != nil {
		session = session.IncludeAttrs(tpm2.AttrAudit)
	}

	pub, _, qualifiedName, err := tpm.ReadPublic(object, session)
	if err != nil {
		var he *tpm2.TPMHandleError
		if xerrors.As(err, &he) && he.Code == tpm2.ErrorHandle {
			return false, nil
		}
		return false, xerrors.Errorf("cannot read public area of object: %w", err)
	}

	pub.Unique = template.Unique

	pubBytes, _ := mu.MarshalToBytes(pub)
	templateBytes, _ := mu.MarshalToBytes(template)
	if !bytes.Equal(pubBytes, templateBytes) {
		// For RSA keys, the default exponent (2^^16 - 1) is normally indicated by the value 0, but handle a TPM that actually
		// returns 65537 by trying again.
		if template.Type == tpm2.ObjectTypeRSA && template.Params.RSADetail.Exponent == 0 {
			var templateCopy *tpm2.Public
			mu.UnmarshalFromBytes(templateBytes, &templateCopy)
			templateCopy.Params.RSADetail.Exponent = 65537
			templateBytes, _ = mu.MarshalToBytes(templateCopy)
			if !bytes.Equal(pubBytes, templateBytes) {
				return false, nil
			}
		} else {
			return false, nil
		}
	}

	// Determine if this is a primary key by validating its qualified name. From the spec, the qualified name
	// of key B (QNb) which is a child of key A is QNb = Hb(QNa || NAMEb). Key A in this case should be
	// the storage primary seed, which has a qualified name matching its name (and the name is the handle
	// for the storage hierarchy)
	h := pub.NameAlg.NewHash()
	h.Write(hierarchy.Name())
	h.Write(object.Name())

	expectedQualifiedName, _ := mu.MarshalToBytes(pub.NameAlg, mu.RawBytes(h.Sum(nil)))
	if !bytes.Equal(expectedQualifiedName, qualifiedName) {
		return false, nil
	}

	return true, nil
}

func bigIntToBytesZeroExtended(x *big.Int, bytes int) (out []byte) {
	b := x.Bytes()
	if len(b) > bytes {
		return b
	}
	out = make([]byte, bytes)
	copy(out[bytes-len(b):], b)
	return
}

// createPublicAreaForECDSAKey creates a *tpm2.Public from a go *ecdsa.PublicKey, which is suitable for loading
// in to a TPM with TPMContext.LoadExternal.
func createTPMPublicAreaForECDSAKey(key *ecdsa.PublicKey) *tpm2.Public {
	var curve tpm2.ECCCurve
	switch key.Curve {
	case elliptic.P224():
		curve = tpm2.ECCCurveNIST_P224
	case elliptic.P256():
		curve = tpm2.ECCCurveNIST_P256
	case elliptic.P384():
		curve = tpm2.ECCCurveNIST_P384
	case elliptic.P521():
		curve = tpm2.ECCCurveNIST_P521
	default:
		panic("unsupported curve")
	}

	return &tpm2.Public{
		Type:    tpm2.ObjectTypeECC,
		NameAlg: tpm2.HashAlgorithmSHA256,
		Attrs:   tpm2.AttrSensitiveDataOrigin | tpm2.AttrUserWithAuth | tpm2.AttrSign,
		Params: &tpm2.PublicParamsU{
			ECCDetail: &tpm2.ECCParams{
				Symmetric: tpm2.SymDefObject{Algorithm: tpm2.SymObjectAlgorithmNull},
				Scheme: tpm2.ECCScheme{
					Scheme:  tpm2.ECCSchemeECDSA,
					Details: &tpm2.AsymSchemeU{ECDSA: &tpm2.SigSchemeECDSA{HashAlg: tpm2.HashAlgorithmSHA256}}},
				CurveID: curve,
				KDF:     tpm2.KDFScheme{Scheme: tpm2.KDFAlgorithmNull}}},
		Unique: &tpm2.PublicIDU{
			ECC: &tpm2.ECCPoint{
				X: bigIntToBytesZeroExtended(key.X, key.Params().BitSize/8),
				Y: bigIntToBytesZeroExtended(key.Y, key.Params().BitSize/8)}}}
}

func createECDSAPrivateKeyFromTPM(public *tpm2.Public, private tpm2.ECCParameter) (*ecdsa.PrivateKey, error) {
	if public.Type != tpm2.ObjectTypeECC {
		return nil, errors.New("unsupported type")
	}

	var curve elliptic.Curve
	switch public.Params.ECCDetail.CurveID {
	case tpm2.ECCCurveNIST_P224:
		curve = elliptic.P224()
	case tpm2.ECCCurveNIST_P256:
		curve = elliptic.P256()
	case tpm2.ECCCurveNIST_P384:
		curve = elliptic.P384()
	case tpm2.ECCCurveNIST_P521:
		curve = elliptic.P521()
	default:
		return nil, errors.New("unsupported curve")
	}

	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(public.Unique.ECC.X),
			Y:     new(big.Int).SetBytes(public.Unique.ECC.Y)},
		D: new(big.Int).SetBytes(private)}, nil
}

// digestListContains indicates whether the specified digest is present in the list of digests.
func digestListContains(list tpm2.DigestList, digest tpm2.Digest) bool {
	for _, d := range list {
		if bytes.Equal(d, digest) {
			return true
		}
	}
	return false
}
