// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/canonical/go-tpm2/internal"
	"github.com/canonical/go-tpm2/mu"

	"golang.org/x/xerrors"
)

type NewCipherFunc func([]byte) (cipher.Block, error)

var (
	eccCurves = map[ECCCurve]elliptic.Curve{
		ECCCurveNIST_P224: elliptic.P224(),
		ECCCurveNIST_P256: elliptic.P256(),
		ECCCurveNIST_P384: elliptic.P384(),
		ECCCurveNIST_P521: elliptic.P521(),
	}

	symmetricAlgs = map[SymAlgorithmId]NewCipherFunc{
		SymAlgorithmAES: aes.NewCipher,
	}
)

// RegisterCipher allows a go block cipher implementation to be registered for the
// specified algorithm, so binaries don't need to link against every implementation.
func RegisterCipher(alg SymAlgorithmId, fn NewCipherFunc) {
	symmetricAlgs[alg] = fn
}

func eccCurveToGoCurve(curve ECCCurve) elliptic.Curve {
	switch curve {
	case ECCCurveNIST_P224:
		return elliptic.P224()
	case ECCCurveNIST_P256:
		return elliptic.P256()
	case ECCCurveNIST_P384:
		return elliptic.P384()
	case ECCCurveNIST_P521:
		return elliptic.P521()
	}
	return nil
}

// ComputeCpHash computes a command parameter digest from the specified command code, the supplied
// handles (identified by their names) and parameter buffer using the specified digest algorithm.
//
// The result of this is useful for extended authorization commands that bind an authorization to
// a command and set of command parameters, such as TPMContext.PolicySigned, TPMContext.PolicySecret,
// TPMContext.PolicyTicket and TPMContext.PolicyCpHash.
//
// This will panic if alg is not available.
func ComputeCpHash(alg HashAlgorithmId, command CommandCode, handles []Name, parameters []byte) Digest {
	hash := alg.NewHash()

	binary.Write(hash, binary.BigEndian, command)
	for _, name := range handles {
		hash.Write([]byte(name))
	}
	hash.Write(parameters)

	return hash.Sum(nil)
}

func cryptComputeRpHash(hashAlg HashAlgorithmId, responseCode ResponseCode, commandCode CommandCode, rpBytes []byte) []byte {
	hash := hashAlg.NewHash()

	binary.Write(hash, binary.BigEndian, responseCode)
	binary.Write(hash, binary.BigEndian, commandCode)
	hash.Write(rpBytes)

	return hash.Sum(nil)
}

func cryptComputeNonce(nonce []byte) error {
	_, err := rand.Read(nonce)
	return err
}

// CryptSymmetricEncrypt performs in place symmetric encryption of the supplied
// data with the specified algorithm using CFB mode.
func CryptSymmetricEncrypt(alg SymAlgorithmId, key, iv, data []byte) error {
	switch alg {
	case SymAlgorithmXOR, SymAlgorithmNull:
		return errors.New("unsupported symmetric algorithm")
	default:
		c, err := alg.NewCipher(key)
		if err != nil {
			return xerrors.Errorf("cannot create cipher: %w", err)
		}
		// The TPM uses CFB cipher mode for all secret sharing
		s := cipher.NewCFBEncrypter(c, iv)
		s.XORKeyStream(data, data)
		return nil
	}
}

// CryptSymmetricDecrypt performs in place symmetric decryption of the supplied
// data with the specified algorithm using CFB mode.
func CryptSymmetricDecrypt(alg SymAlgorithmId, key, iv, data []byte) error {
	switch alg {
	case SymAlgorithmXOR, SymAlgorithmNull:
		return errors.New("unsupported symmetric algorithm")
	default:
		c, err := alg.NewCipher(key)
		if err != nil {
			return xerrors.Errorf("cannot create cipher: %w", err)
		}
		// The TPM uses CFB cipher mode for all secret sharing
		s := cipher.NewCFBDecrypter(c, iv)
		s.XORKeyStream(data, data)
		return nil
	}
}

func zeroExtendBytes(x *big.Int, l int) (out []byte) {
	out = make([]byte, l)
	tmp := x.Bytes()
	copy(out[len(out)-len(tmp):], tmp)
	return
}

// CryptSecretEncrypt creates a secret value and its associated secret structure using
// the asymmetric algorithm defined by public. This is useful for sharing secrets with
// the TPM via the TPMContext.Import and TPMContext.ActivateCredential functions.
//
// It is also used internally by TPMContext.StartAuthSession.
func CryptSecretEncrypt(public *Public, label []byte) (EncryptedSecret, []byte, error) {
	if !public.NameAlg.Available() {
		return nil, nil, fmt.Errorf("nameAlg %v is not available", public.NameAlg)
	}
	digestSize := public.NameAlg.Size()

	switch public.Type {
	case ObjectTypeRSA:
		if public.Params.RSADetail.Scheme.Scheme != RSASchemeNull &&
			public.Params.RSADetail.Scheme.Scheme != RSASchemeOAEP {
			return nil, nil, errors.New("unsupported RSA scheme")
		}
		pub := public.Public().(*rsa.PublicKey)

		secret := make([]byte, digestSize)
		if _, err := rand.Read(secret); err != nil {
			return nil, nil, fmt.Errorf("cannot read random bytes for secret: %v", err)
		}

		h := public.NameAlg.NewHash()
		label0 := make([]byte, len(label)+1)
		copy(label0, label)
		encryptedSecret, err := rsa.EncryptOAEP(h, rand.Reader, pub, secret, label0)
		return encryptedSecret, secret, err
	case ObjectTypeECC:
		pub := public.Public().(*ecdsa.PublicKey)
		if pub.Curve == nil {
			return nil, nil, fmt.Errorf("unsupported curve: %v", public.Params.ECCDetail.CurveID.GoCurve())
		}
		if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
			return nil, nil, fmt.Errorf("public key is not on curve")
		}

		ephPriv, ephX, ephY, err := elliptic.GenerateKey(pub.Curve, rand.Reader)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot generate ephemeral ECC key: %v", err)
		}

		sz := pub.Curve.Params().BitSize / 8

		encryptedSecret, err := mu.MarshalToBytes(&ECCPoint{
			X: zeroExtendBytes(ephX, sz),
			Y: zeroExtendBytes(ephY, sz)})
		if err != nil {
			panic(fmt.Sprintf("failed to marshal secret: %v", err))
		}

		mulX, _ := pub.Curve.ScalarMult(pub.X, pub.Y, ephPriv)
		secret := internal.KDFe(public.NameAlg.GetHash(),
			zeroExtendBytes(mulX, sz), label, zeroExtendBytes(ephX, sz),
			zeroExtendBytes(pub.X, sz), digestSize*8)
		return encryptedSecret, secret, nil
	default:
		return nil, nil, fmt.Errorf("unsupported key type %v", public.Type)
	}
}
