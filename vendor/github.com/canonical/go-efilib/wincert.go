// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"

	"github.com/canonical/go-efilib/internal/ioerr"
	"github.com/canonical/go-efilib/internal/pkcs7"
	"github.com/canonical/go-efilib/internal/uefi"
)

var (
	oidSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}

	oidSpcIndirectData   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 4}
	oidSpcPeImageDataobj = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 15}
)

func readAlgorithmIdentifier(der cryptobyte.String) (*pkix.AlgorithmIdentifier, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	ai := new(pkix.AlgorithmIdentifier)

	if !der.ReadASN1ObjectIdentifier(&ai.Algorithm) {
		return nil, errors.New("malformed algorithm")
	}

	if der.Empty() {
		return ai, nil
	}

	var paramsBytes cryptobyte.String
	var paramsTag cryptobyte_asn1.Tag
	if !der.ReadAnyASN1Element(&paramsBytes, &paramsTag) {
		return nil, errors.New("malformed parameters")
	}
	ai.Parameters.Class = int(paramsTag & 0xc0)
	ai.Parameters.Tag = int(paramsTag & 0x1f)
	if paramsTag&0x20 != 0 {
		ai.Parameters.IsCompound = true
	}
	ai.Parameters.FullBytes = paramsBytes
	paramsBytes.ReadASN1((*cryptobyte.String)(&ai.Parameters.Bytes), paramsTag)

	return ai, nil
}

type spcPeImageData struct {
	flags asn1.BitString
	// file spcLink
}

func readSpcPeImageData(der cryptobyte.String) (*spcPeImageData, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	data := new(spcPeImageData)

	if !der.ReadASN1BitString(&data.flags) {
		return nil, errors.New("malformed flags")
	}

	return data, nil
}

type digestInfo struct {
	digestAlgorithm pkix.AlgorithmIdentifier
	digest          []byte
}

func readDigestInfo(der cryptobyte.String) (*digestInfo, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	di := new(digestInfo)

	var daRaw cryptobyte.String
	if !der.ReadASN1Element(&daRaw, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed digestAlgorithm")
	}
	da, err := readAlgorithmIdentifier(daRaw)
	if err != nil {
		return nil, fmt.Errorf("cannot read digestAlgorithm: %w", err)
	}
	di.digestAlgorithm = *da

	if !der.ReadASN1((*cryptobyte.String)(&di.digest), cryptobyte_asn1.OCTET_STRING) {
		return nil, errors.New("malformed digest")
	}

	return di, nil

}

type spcAttributeTypeAndOptionalValue struct {
	attrType asn1.ObjectIdentifier
	valueRaw cryptobyte.String
}

func readSpcAttributeTypeAndOptionalValue(der cryptobyte.String) (*spcAttributeTypeAndOptionalValue, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	data := new(spcAttributeTypeAndOptionalValue)

	if !der.ReadASN1ObjectIdentifier(&data.attrType) {
		return nil, errors.New("malformed type")
	}

	if der.Empty() {
		return data, nil
	}

	// This is weird - the spec documents this field with:
	//  value [0] EXPLICIT ANY OPTIONAL
	//
	// It's not explicit though - the underlying SpcPeImageData structure
	// doesn't have another tag. The tag in public signatures is actually
	// just a universal sequence tag. I think this should really
	// be:
	//  value ANY DEFINED BY type OPTIONAL
	if !der.ReadAnyASN1Element(&data.valueRaw, nil) {
		return nil, errors.New("malformed value")
	}
	return data, nil
}

type spcIndirectDataContent struct {
	data          spcAttributeTypeAndOptionalValue
	messageDigest digestInfo
}

func readSpcIndirectDataContent(der cryptobyte.String) (*spcIndirectDataContent, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	idc := new(spcIndirectDataContent)

	var dataRaw cryptobyte.String
	if !der.ReadASN1Element(&dataRaw, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed data")
	}
	data, err := readSpcAttributeTypeAndOptionalValue(dataRaw)
	if err != nil {
		return nil, fmt.Errorf("cannot read data: %w", err)
	}
	idc.data = *data

	var mdRaw cryptobyte.String
	if !der.ReadASN1Element(&mdRaw, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed messageDigest")
	}
	md, err := readDigestInfo(mdRaw)
	if err != nil {
		return nil, fmt.Errorf("cannot read messageDigest: %w", err)
	}
	idc.messageDigest = *md

	return idc, nil
}

type authenticodeContent struct {
	digestAlgorithm asn1.ObjectIdentifier
	flags           asn1.BitString
	digest          []byte
}

func unmarshalAuthenticodeContent(data []byte) (*authenticodeContent, error) {
	idc, err := readSpcIndirectDataContent(cryptobyte.String(data))
	if err != nil {
		return nil, fmt.Errorf("cannot read SpcIndirectDataContent: %w", err)
	}

	if !idc.data.attrType.Equal(oidSpcPeImageDataobj) {
		return nil, errors.New("not a PE image data object")
	}

	peData, err := readSpcPeImageData(idc.data.valueRaw)
	if err != nil {
		return nil, fmt.Errorf("cannot read spcPeImageData: %w", err)
	}

	return &authenticodeContent{
		digestAlgorithm: idc.messageDigest.digestAlgorithm.Algorithm,
		flags:           peData.flags,
		digest:          idc.messageDigest.digest}, nil
}

func certLikelyIssued(issuer, subject *x509.Certificate) bool {
	if !bytes.Equal(issuer.RawSubject, subject.RawIssuer) {
		return false
	}

	if !bytes.Equal(issuer.SubjectKeyId, subject.AuthorityKeyId) {
		// XXX: this ignores the issuer and serial number fields
		// of the akid extension, although crypto/x509 doesn't
		// expose this - we'd have to parse it ourselves.
		return false
	}

	switch issuer.PublicKeyAlgorithm {
	case x509.RSA:
		switch subject.SignatureAlgorithm {
		case x509.SHA1WithRSA, x509.SHA256WithRSA, x509.SHA384WithRSA, x509.SHA512WithRSA:
			return true
		case x509.SHA256WithRSAPSS, x509.SHA384WithRSAPSS, x509.SHA512WithRSAPSS:
			return true
		default:
			return false
		}
	case x509.ECDSA:
		switch subject.SignatureAlgorithm {
		case x509.ECDSAWithSHA1, x509.ECDSAWithSHA256, x509.ECDSAWithSHA384, x509.ECDSAWithSHA512:
			return true
		default:
			return false
		}
	case x509.Ed25519:
		switch subject.SignatureAlgorithm {
		case x509.PureEd25519:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isSelfSignedCert(cert *x509.Certificate) bool {
	return certLikelyIssued(cert, cert)
}

func certsMatch(x, y *x509.Certificate) bool {
	return bytes.Equal(x.RawSubject, y.RawSubject) &&
		bytes.Equal(x.SubjectKeyId, y.SubjectKeyId) &&
		x.SignatureAlgorithm == y.SignatureAlgorithm &&
		bytes.Equal(x.RawIssuer, y.RawIssuer) &&
		bytes.Equal(x.AuthorityKeyId, y.AuthorityKeyId) &&
		x.PublicKeyAlgorithm == y.PublicKeyAlgorithm
}

func buildCertChains(trusted *x509.Certificate, untrusted []*x509.Certificate, chain []*x509.Certificate, depth *int) (chains [][]*x509.Certificate) {
	removeCert := func(certs []*x509.Certificate, x *x509.Certificate) []*x509.Certificate {
		var newCerts []*x509.Certificate
		for _, cert := range certs {
			if cert == x {
				continue
			}
			newCerts = append(newCerts, cert)
		}
		return newCerts
	}

	if depth == nil {
		depth = new(int)
	}
	*depth++
	if *depth > 100 {
		return nil
	}

	current := chain[len(chain)-1]

	if !isSelfSignedCert(current) {
		// for certificates that aren't self-signed:
		// check the list of untrusted certs first
		for _, x := range untrusted {
			if !certLikelyIssued(x, current) {
				continue
			}
			// try to build chains with this untrusted cert
			chains = append(chains, buildCertChains(trusted, removeCert(untrusted, x), append(chain, x), depth)...)
		}

		// check the trust anchor
		if certLikelyIssued(trusted, current) {
			// we have a complete chain
			chains = append(chains, append(chain, trusted))
		}
	}

	// If we have no chains, check if the current certificate is the
	// trust anchor. This handles the case where the leaf certificate
	// is the trust anchor. We should only reach this condition at
	// depth==1. Checking that there are no chains before comparing
	// is an optimization because if there are then we know that
	// current != trusted.
	if len(chains) == 0 && certsMatch(trusted, current) {
		chains = append(chains, chain)
	}

	return chains
}

type WinCertificateType uint16

const (
	// WinCertificateTypeAuthenticode indicates that a WinCertificate
	// is an authenticode signature and is implemented by the
	// *WinCertificateAuthenticode type.
	WinCertificateTypeAuthenticode WinCertificateType = uefi.WIN_CERT_TYPE_PKCS_SIGNED_DATA

	// WinCertificatePKCS1v15 indicates that a WinCertificate is a
	// PKCS#1-v1.5 encoded RSA2048 signature and is implemented by
	// the *WinCertificatePKCS1v15 type.
	WinCertificateTypePKCS1v15 WinCertificateType = uefi.WIN_CERT_TYPE_EFI_PKCS115

	// WinCertificateTypeGUID indicates that a WinCertificate is a
	// signature of a type indicated by a separate GUID and is implemented
	// by a type that implements the WinCertificateGUID interface.
	WinCertificateTypeGUID WinCertificateType = uefi.WIN_CERT_TYPE_EFI_GUID
)

// WinCertificate is an interface type corresponding to implementations of WIN_CERTIFICATE.
type WinCertificate interface {
	Type() WinCertificateType // Type of this certificate
}

// WinCertificatePKCS1v15 corresponds to the WIN_CERTIFICATE_EFI_PKCS1_15 type
// and represents a RSA2048 signature with PKCS#1 v1.5 padding.
type WinCertificatePKCS1v15 struct {
	HashAlgorithm crypto.Hash
	Signature     [256]byte
}

func (c *WinCertificatePKCS1v15) Type() WinCertificateType {
	return WinCertificateTypePKCS1v15
}

// WinCertificateGUID corresponds to implementations of WIN_CERTIFICATE_UEFI_GUID.
type WinCertificateGUID interface {
	WinCertificate
	GUIDType() GUID
}

func newWinCertificateGUID(cert *uefi.WIN_CERTIFICATE_UEFI_GUID) (WinCertificateGUID, error) {
	switch cert.CertType {
	case uefi.EFI_CERT_TYPE_RSA2048_SHA256_GUID:
		if len(cert.CertData) != binary.Size(WinCertificateGUIDPKCS1v15{}) {
			return nil, errors.New("invalid length for WIN_CERTIFICATE_UEFI_GUID with EFI_CERT_TYPE_RSA2048_SHA256_GUID type")
		}
		c := new(WinCertificateGUIDPKCS1v15)
		binary.Read(bytes.NewReader(cert.CertData), binary.LittleEndian, &c)
		return c, nil
	case uefi.EFI_CERT_TYPE_PKCS7_GUID:
		p7, err := pkcs7.UnmarshalSignedData(cert.CertData)
		if err != nil {
			return nil, fmt.Errorf("cannot decode payload for WIN_CERTIFICATE_UEFI_GUID with EFI_CERT_TYPE_PKCS7_GUID type: %w", err)
		}
		return &WinCertificatePKCS7{p7: p7}, nil
	default:
		return &WinCertificateGUIDUnknown{unknownGUIDType: GUID(cert.CertType), Data: cert.CertData}, nil
	}
}

// WinCertificateGUIDUnknown corresponds to a WIN_CERTIFICATE_UEFI_GUID with
// an unknown type.
type WinCertificateGUIDUnknown struct {
	unknownGUIDType GUID
	Data            []byte
}

func (c *WinCertificateGUIDUnknown) Type() WinCertificateType {
	return WinCertificateTypeGUID
}

func (c *WinCertificateGUIDUnknown) GUIDType() GUID {
	return c.unknownGUIDType
}

// WinCertificateGUIDPKCS1v15 corresponds to a WIN_CERTIFICATE_UEFI_GUID with
// the EFI_CERT_TYPE_RSA2048_SHA256_GUID type, and represents a RSA2048 SHA256
// signature with PKCS#1 v1.5 padding
type WinCertificateGUIDPKCS1v15 struct {
	PublicKey [256]byte
	Signature [256]byte
}

func (c *WinCertificateGUIDPKCS1v15) Type() WinCertificateType {
	return WinCertificateTypeGUID
}

func (c *WinCertificateGUIDPKCS1v15) GUIDType() GUID {
	return CertTypeRSA2048SHA256Guid
}

// WinCertificatePKCS7 corresponds to a WIN_CERTIFICATE_UEFI_GUID with
// the EFI_CERT_TYPE_PKCS7_GUID type, and represents a detached PKCS7
// signature.
type WinCertificatePKCS7 struct {
	p7 *pkcs7.SignedData
}

func (c *WinCertificatePKCS7) Type() WinCertificateType {
	return WinCertificateTypeGUID
}

func (c *WinCertificatePKCS7) GUIDType() GUID {
	return CertTypePKCS7Guid
}

// GetSigners returns the signing certificates.
func (c *WinCertificatePKCS7) GetSigners() []*x509.Certificate {
	return c.p7.GetSigners()
}

// CertLikelyTrustAnchor determines if the specified certificate is likely to be
// a trust anchor for this signature. This is "likely" because it only checks if
// there are candidate certificate chains rooted to the specified certificate.
// When attempting to build candidate certificate chains, it considers a certificate
// to be likely issued by another certificate if:
//   - The certificate's issuer matches the issuer's subject.
//   - The certificate's Authority Key Identifier keyIdentifier field matches the
//     issuer's Subject Key Identifier.
//   - The certificate's signature algorithm is compatible with the issuer's public
//     key algorithm.
//
// It performs no verification of any candidate certificate chains and no verification
// of the signature.
func (c *WinCertificatePKCS7) CertLikelyTrustAnchor(cert *x509.Certificate) bool {
	for _, s := range c.GetSigners() {
		if len(buildCertChains(cert, c.p7.Certificates, []*x509.Certificate{s}, nil)) == 0 {
			return false
		}
	}

	return true
}

// WinCertificateAuthenticode corresponds to a WIN_CERTIFICATE_EFI_PKCS and
// represents an Authenticode signature.
type WinCertificateAuthenticode struct {
	p7           *pkcs7.SignedData
	authenticode *authenticodeContent
}

func (c *WinCertificateAuthenticode) Type() WinCertificateType {
	return WinCertificateTypeAuthenticode
}

// GetSigner returns the signing certificate.
func (c *WinCertificateAuthenticode) GetSigner() *x509.Certificate {
	return c.p7.GetSigners()[0]
}

// CertLikelyTrustAnchor determines if the specified certificate is likely to be
// a trust anchor for this signature. This is "likely" because it only checks if
// there are candidate certificate chains rooted to the specified certificate.
// When attempting to build candidate certificate chains, it considers a certificate
// to be likely issued by another certificate if:
//   - The certificate's issuer matches the issuer's subject.
//   - The certificate's Authority Key Identifier keyIdentifier field matches the
//     issuer's Subject Key Identifier.
//   - The certificate's signature algorithm is compatible with the issuer's public
//     key algorithm.
//
// It performs no verification of any candidate certificate chains and no verification
// of the signature.
func (c *WinCertificateAuthenticode) CertLikelyTrustAnchor(cert *x509.Certificate) bool {
	return len(buildCertChains(cert, c.p7.Certificates, []*x509.Certificate{c.GetSigner()}, nil)) > 0
}

func (c *WinCertificateAuthenticode) DigestAlgorithm() crypto.Hash {
	switch {
	case c.authenticode.digestAlgorithm.Equal(oidSHA256):
		return crypto.SHA256
	default:
		return crypto.Hash(0)
	}
}

// Digest returns the PE image digest of the image associated with this
// signature.
func (c *WinCertificateAuthenticode) Digest() []byte {
	return c.authenticode.digest
}

func digestAlgorithmIdToCryptoHash(id GUID) (crypto.Hash, error) {
	switch id {
	case HashAlgorithmSHA1Guid:
		return crypto.SHA1, nil
	case HashAlgorithmSHA256Guid:
		return crypto.SHA256, nil
	case HashAlgorithmSHA224Guid:
		return crypto.SHA224, nil
	case HashAlgorithmSHA384Guid:
		return crypto.SHA384, nil
	case HashAlgorithmSHA512Guid:
		return crypto.SHA512, nil
	default:
		return crypto.Hash(0), errors.New("unrecognized digest")
	}
}

// ReadWinCertificate decodes a signature (something that is confusingly
// represented by types with "certificate" in the name in both the UEFI
// and PE/COFF specifications) from the supplied reader and returns a
// WinCertificate of the appropriate type. The type returned is dependent
// on the data, and will be one of *[WinCertificateAuthenticode],
// *[WinCertificatePKCS1v15], *[WinCertificatePKCS7] or
// *[WinCertificateGUIDPKCS1v15].
func ReadWinCertificate(r io.Reader) (WinCertificate, error) {
	var hdr uefi.WIN_CERTIFICATE
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, err
	}
	if hdr.Revision != 0x0200 {
		return nil, errors.New("unexpected revision")
	}

	switch hdr.CertificateType {
	case uefi.WIN_CERT_TYPE_PKCS_SIGNED_DATA:
		cert := uefi.WIN_CERTIFICATE_EFI_PKCS{Hdr: hdr}
		cert.CertData = make([]byte, int(cert.Hdr.Length)-binary.Size(cert.Hdr))
		if _, err := io.ReadFull(r, cert.CertData); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS: %w", err)
		}

		p7, err := pkcs7.UnmarshalSignedData(cert.CertData)
		if err != nil {
			return nil, fmt.Errorf("cannot decode WIN_CERTIFICATE_EFI_PKCS payload: %w", err)
		}
		if len(p7.GetSigners()) != 1 {
			return nil, errors.New("WIN_CERTIFICATE_EFI_PKCS has invalid number of signers")
		}

		if !p7.ContentType().Equal(oidSpcIndirectData) {
			return nil, errors.New("WIN_CERTIFICATE_EFI_PKCS has invalid content type")
		}
		auth, err := unmarshalAuthenticodeContent(p7.Content())
		if err != nil {
			return nil, fmt.Errorf("cannot decode authenticode content for WIN_CERTIFICATE_EFI_PKCS: %w", err)
		}
		return &WinCertificateAuthenticode{p7: p7, authenticode: auth}, nil
	case uefi.WIN_CERT_TYPE_EFI_PKCS115:
		if hdr.Length != uint32(binary.Size(uefi.WIN_CERTIFICATE_EFI_PKCS1_15{})) {
			return nil, fmt.Errorf("invalid length for WIN_CERTIFICATE_EFI_PKCS1_15: %d", hdr.Length)
		}

		cert := uefi.WIN_CERTIFICATE_EFI_PKCS1_15{Hdr: hdr}
		if _, err := io.ReadFull(r, cert.HashAlgorithm[:]); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS1_15: %w", err)
		}
		if _, err := io.ReadFull(r, cert.Signature[:]); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS1_15: %w", err)
		}

		digest, err := digestAlgorithmIdToCryptoHash(GUID(cert.HashAlgorithm))
		if err != nil {
			return nil, fmt.Errorf("cannot determine digest algorithm for WIN_CERTIFICATE_EFI_PKCS1_15: %w", err)
		}

		return &WinCertificatePKCS1v15{HashAlgorithm: digest, Signature: cert.Signature}, nil
	case uefi.WIN_CERT_TYPE_EFI_GUID:
		cert := uefi.WIN_CERTIFICATE_UEFI_GUID{Hdr: hdr}
		cert.CertData = make([]byte, int(cert.Hdr.Length)-binary.Size(cert.Hdr)-binary.Size(cert.CertType))
		if _, err := io.ReadFull(r, cert.CertType[:]); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_UEFI_GUID: %w", err)
		}
		if _, err := io.ReadFull(r, cert.CertData); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_UEFI_GUID: %w", err)
		}
		return newWinCertificateGUID(&cert)
	default:
		return nil, errors.New("unexpected type")
	}
}
