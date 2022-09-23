// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package util

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"math/big"

	"golang.org/x/xerrors"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/internal"
	"github.com/canonical/go-tpm2/mu"
)

func zeroExtendBytes(x *big.Int, l int) (out []byte) {
	out = make([]byte, l)
	tmp := x.Bytes()
	copy(out[len(out)-len(tmp):], tmp)
	return
}

// CryptSecretDecrypt recovers a secret value from the supplied secret structure
// using the private key. It can be used to recover secrets created by the TPM,
// such as those created by the TPM2_Duplicate command.
func CryptSecretDecrypt(priv crypto.PrivateKey, hashAlg tpm2.HashAlgorithmId, label []byte, secret tpm2.EncryptedSecret) ([]byte, error) {
	if !hashAlg.Available() {
		return nil, errors.New("digest algorithm is not available")
	}

	switch p := priv.(type) {
	case *rsa.PrivateKey:
		h := hashAlg.NewHash()
		label0 := make([]byte, len(label)+1)
		copy(label0, label)
		return rsa.DecryptOAEP(h, rand.Reader, p, secret, label0)
	case *ecdsa.PrivateKey:
		var ephPoint tpm2.ECCPoint
		if _, err := mu.UnmarshalFromBytes(secret, &ephPoint); err != nil {
			return nil, xerrors.Errorf("cannot unmarshal ephemeral point: %w", err)
		}
		ephX := new(big.Int).SetBytes(ephPoint.X)
		ephY := new(big.Int).SetBytes(ephPoint.Y)

		if !p.Curve.IsOnCurve(ephX, ephY) {
			return nil, errors.New("ephemeral point is not on curve")
		}

		sz := p.Curve.Params().BitSize / 8

		mulX, _ := p.Curve.ScalarMult(ephX, ephY, p.D.Bytes())
		return internal.KDFe(hashAlg.GetHash(), zeroExtendBytes(mulX, sz), label,
			ephPoint.X, zeroExtendBytes(p.X, sz), hashAlg.Size()*8), nil
	default:
		return nil, errors.New("unsupported key type")
	}
}
