// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package util

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"
)

// Sign creates a signature of the supplied digest using the supplied private key and
// signature scheme. Note that only RSA-SSA, RSA-PSS, ECDSA and HMAC signatures can
// be created. The returned signature can be verified on a TPM using the associated
// public key.
//
// In order to create a HMAC, the supplied private key should be a byte slice containing
// the HMAC key.
func Sign(key crypto.PrivateKey, scheme *tpm2.SigScheme, digest []byte) (*tpm2.Signature, error) {
	hashAlg := scheme.Details.Any(scheme.Scheme).HashAlg
	if !hashAlg.Available() {
		return nil, errors.New("digest algorithm is not available")
	}

	if len(digest) != hashAlg.Size() {
		return nil, errors.New("invalid digest length")
	}

	switch k := key.(type) {
	case *rsa.PrivateKey:
		switch scheme.Scheme {
		case tpm2.SigSchemeAlgRSASSA:
			sig, err := rsa.SignPKCS1v15(rand.Reader, k, hashAlg.GetHash(), digest)
			if err != nil {
				return nil, err
			}

			return &tpm2.Signature{
				SigAlg: tpm2.SigSchemeAlgRSASSA,
				Signature: &tpm2.SignatureU{
					RSASSA: &tpm2.SignatureRSASSA{
						Hash: hashAlg,
						Sig:  sig}}}, nil
		case tpm2.SigSchemeAlgRSAPSS:
			options := rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}
			sig, err := rsa.SignPSS(rand.Reader, k, hashAlg.GetHash(), digest, &options)
			if err != nil {
				return nil, err
			}

			return &tpm2.Signature{
				SigAlg: tpm2.SigSchemeAlgRSAPSS,
				Signature: &tpm2.SignatureU{
					RSAPSS: &tpm2.SignatureRSAPSS{
						Hash: hashAlg,
						Sig:  sig}}}, nil
		default:
			return nil, errors.New("unsupported RSA signature scheme")
		}
	case *ecdsa.PrivateKey:
		switch scheme.Scheme {
		case tpm2.SigSchemeAlgECDSA:
			r, s, err := ecdsa.Sign(rand.Reader, k, digest)
			if err != nil {
				return nil, err
			}

			return &tpm2.Signature{
				SigAlg: tpm2.SigSchemeAlgECDSA,
				Signature: &tpm2.SignatureU{
					ECDSA: &tpm2.SignatureECDSA{
						Hash:       hashAlg,
						SignatureR: r.Bytes(),
						SignatureS: s.Bytes()}}}, nil
		default:
			return nil, errors.New("unsupported ECC signature scheme")
		}
	case []byte:
		switch scheme.Scheme {
		case tpm2.SigSchemeAlgHMAC:
			h := hmac.New(hashAlg.NewHash, k)
			h.Write(digest)

			return &tpm2.Signature{
				SigAlg: tpm2.SigSchemeAlgHMAC,
				Signature: &tpm2.SignatureU{
					HMAC: &tpm2.TaggedHash{
						HashAlg: hashAlg,
						Digest:  h.Sum(nil)}}}, nil
		default:
			return nil, errors.New("unsupported keyed hash scheme")
		}
	default:
		return nil, errors.New("unsupported private key type")
	}
}

// VerifySignature verifies a signature created by a TPM using the supplied public
// key. Note that only RSA-SSA, RSA-PSS, ECDSA and HMAC signatures are supported.
//
// In order to verify a HMAC signature, the supplied public key should be a byte
// slice containing the HMAC key.
func VerifySignature(key crypto.PublicKey, digest []byte, signature *tpm2.Signature) (ok bool, err error) {
	if !signature.Signature.Any(signature.SigAlg).HashAlg.Available() {
		return false, errors.New("digest algorithm is not available")
	}

	switch k := key.(type) {
	case *rsa.PublicKey:
		switch signature.SigAlg {
		case tpm2.SigSchemeAlgRSASSA:
			if err := rsa.VerifyPKCS1v15(k, signature.Signature.RSASSA.Hash.GetHash(), digest, signature.Signature.RSASSA.Sig); err != nil {
				if err == rsa.ErrVerification {
					return false, nil
				}
				return false, err
			}
			return true, nil
		case tpm2.SigSchemeAlgRSAPSS:
			options := rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}
			if err := rsa.VerifyPSS(k, signature.Signature.RSAPSS.Hash.GetHash(), digest, signature.Signature.RSAPSS.Sig, &options); err != nil {
				if err == rsa.ErrVerification {
					return false, nil
				}
				return false, err
			}
			return true, nil
		default:
			return false, errors.New("unsupported RSA signature algorithm")
		}
	case *ecdsa.PublicKey:
		switch signature.SigAlg {
		case tpm2.SigSchemeAlgECDSA:
			ok = ecdsa.Verify(k, digest, new(big.Int).SetBytes(signature.Signature.ECDSA.SignatureR),
				new(big.Int).SetBytes(signature.Signature.ECDSA.SignatureS))
			return ok, nil
		default:
			return false, errors.New("unsupported ECC signature algorithm")
		}
	case []byte:
		switch signature.SigAlg {
		case tpm2.SigSchemeAlgHMAC:
			scheme := &tpm2.SigScheme{
				Scheme: tpm2.SigSchemeAlgHMAC,
				Details: &tpm2.SigSchemeU{
					HMAC: &tpm2.SchemeHMAC{
						HashAlg: signature.Signature.HMAC.HashAlg}}}
			test, err := Sign(k, scheme, digest)
			if err != nil {
				return false, err
			}
			return bytes.Equal(signature.Signature.HMAC.Digest, test.Signature.HMAC.Digest), nil
		default:
			return false, errors.New("unsupported keyed hash signature algorithm")
		}
	default:
		return false, errors.New("invalid public key type")
	}
}

// SignPolicyAuthorization creates a signed authorization using the supplied key and signature
// scheme. The signed authorization can be used in a TPM2_PolicySigned assertion. The authorizing
// party can apply contraints on how the session that includes this authorization can be used.
//
// If nonceTPM is supplied, then the signed authorization can only be used for the session
// associated with the supplied nonce.
//
// If expiration is non-zero, then the signed authorization is only valid for the specified
// number of seconds from when nonceTPM was generated.
//
// If cpHash is supplied, then the signed authorization is only valid for use in a command
// with the associated set of command parameters.
func SignPolicyAuthorization(key crypto.PrivateKey, scheme *tpm2.SigScheme, nonceTPM tpm2.Nonce, cpHashA tpm2.Digest, policyRef tpm2.Nonce, expiration int32) (*tpm2.Signature, error) {
	hashAlg := scheme.Details.Any(scheme.Scheme).HashAlg
	if !hashAlg.Available() {
		return nil, errors.New("digest algorithm is not available")
	}

	h := hashAlg.NewHash()
	h.Write(nonceTPM)
	binary.Write(h, binary.BigEndian, expiration)
	h.Write(cpHashA)
	h.Write(policyRef)

	return Sign(key, scheme, h.Sum(nil))
}

// ComputePolicyAuthorizeDigest computes a digest to sign from the supplied
// authorization policy digest and policy reference. The resulting digest
// can be signed to authorize the supplied policy with the TPM2_PolicyAuthorize
// assertion.
func ComputePolicyAuthorizeDigest(alg tpm2.HashAlgorithmId, approvedPolicy tpm2.Digest, policyRef tpm2.Nonce) (tpm2.Digest, error) {
	if !alg.Available() {
		return nil, errors.New("digest algorithm is not available")
	}

	h := alg.NewHash()
	h.Write(approvedPolicy)
	h.Write(policyRef)
	return h.Sum(nil), nil
}

// PolicyAuthorize authorizes an authorization policy digest with the supplied key and
// signature scheme. The resulting digest and signature can be verified by the TPM in
// order to produce a ticket that can then be supplied to a TPM2_PolicyAuthorize assertion.
//
// The digest algorithm used for the signature must match the name algorithm in
// the public area associated with the supplied private key.
func PolicyAuthorize(key crypto.PrivateKey, scheme *tpm2.SigScheme, approvedPolicy tpm2.Digest, policyRef tpm2.Nonce) (tpm2.Digest, *tpm2.Signature, error) {
	hashAlg := scheme.Details.Any(scheme.Scheme).HashAlg
	if !hashAlg.Available() {
		return nil, nil, errors.New("digest algorithm is not available")
	}

	digest, _ := ComputePolicyAuthorizeDigest(hashAlg, approvedPolicy, policyRef)
	sig, err := Sign(key, scheme, digest)
	if err != nil {
		return nil, nil, err
	}

	return digest, sig, nil
}

// VerifyAttestationSignature verifies the signature for the supplied attestation
// structure as generated by one of the TPM's attestation commands. Note that only
// RSA-SSA, RSA-PSS, ECDSA and HMAC signatures are supported.
//
// In order to verify a HMAC signature, the supplied public key should be a byte
// slice containing the HMAC key.
func VerifyAttestationSignature(key crypto.PublicKey, attest *tpm2.Attest, signature *tpm2.Signature) (ok bool, err error) {
	hashAlg := signature.Signature.Any(signature.SigAlg).HashAlg
	if !hashAlg.Available() {
		return false, errors.New("digest algorithm is not available")
	}

	h := hashAlg.NewHash()
	mu.MustMarshalToWriter(h, attest)

	return VerifySignature(key, h.Sum(nil), signature)
}
