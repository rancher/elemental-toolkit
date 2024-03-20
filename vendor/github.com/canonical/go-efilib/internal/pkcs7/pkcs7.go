// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package pkcs7

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"

	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"
)

var (
	oidSignedData = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
)

type contentInfo struct {
	contentType asn1.ObjectIdentifier
	content     []byte
}

func readContentInfo(der cryptobyte.String) (*contentInfo, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	ci := new(contentInfo)

	if !der.ReadASN1ObjectIdentifier(&ci.contentType) {
		return nil, errors.New("malformed contentType")
	}

	if !der.ReadOptionalASN1((*cryptobyte.String)(&ci.content), nil, cryptobyte_asn1.Tag(0).ContextSpecific().Constructed()) {
		return nil, errors.New("malformed content")
	}

	return ci, nil
}

type issuerAndSerialNumber struct {
	issuerNameRaw []byte
	serialNumber  big.Int
}

func readIssuerAndSerialNumber(der cryptobyte.String) (*issuerAndSerialNumber, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	isn := new(issuerAndSerialNumber)

	if !der.ReadASN1Element((*cryptobyte.String)(&isn.issuerNameRaw), cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed issuerName")
	}

	if !der.ReadASN1Integer(&isn.serialNumber) {
		return nil, errors.New("malformed serialNumber")
	}

	return isn, nil

}

type signerInfo struct {
	version               int
	issuerAndSerialNumber issuerAndSerialNumber
	// digestAlgorithm pkix.AlgorithmIdentifier
	// authenticatedAttributes []attribute
	// digestEncryptionAlgorithm pkix.AlgorithmIdentifier
	// encryptedDigest []byte
	// unauthenticatedAttriubtes []attribute
}

func readSignerInfo(der cryptobyte.String) (*signerInfo, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	si := new(signerInfo)

	if !der.ReadASN1Integer(&si.version) {
		return nil, errors.New("malformed version")
	}

	var isnRaw cryptobyte.String
	if !der.ReadASN1Element(&isnRaw, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed issuerAndSerialNumber")
	}
	isn, err := readIssuerAndSerialNumber(isnRaw)
	if err != nil {
		return nil, fmt.Errorf("cannot read issuerAndSerialNumber: %w", err)
	}
	si.issuerAndSerialNumber = *isn

	return si, nil
}

type signedData struct {
	version int
	//digestAlgorithms []pkix.AlgorithmIdentifier
	contentInfo  contentInfo
	certificates []*x509.Certificate
	//crls		 []pkix.RevokedCertificate
	signerInfos []signerInfo
}

func readSignedData(der cryptobyte.String) (*signedData, error) {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed input")
	}

	sd := new(signedData)

	sd.version = 1
	if der.PeekASN1Tag(cryptobyte_asn1.INTEGER) {
		if !der.ReadASN1Integer(&sd.version) {
			return nil, errors.New("malformed version")
		}
	}
	if sd.version != 1 {
		return nil, fmt.Errorf("invalid version %d", sd.version)
	}

	var unused cryptobyte.String
	if !der.ReadASN1(&unused, cryptobyte_asn1.SET) {
		return nil, errors.New("malformed digestAlgorithms")
	}

	var ciRaw cryptobyte.String
	if !der.ReadASN1Element(&ciRaw, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("malformed contentInfo")
	}
	ci, err := readContentInfo(ciRaw)
	if err != nil {
		return nil, fmt.Errorf("cannot read contentInfo: %w", err)
	}
	sd.contentInfo = *ci

	var certsRaw cryptobyte.String
	if !der.ReadOptionalASN1(&certsRaw, nil, cryptobyte_asn1.Tag(0).ContextSpecific().Constructed()) {
		return nil, errors.New("malformed certificates")
	}
	certs, err := x509.ParseCertificates(certsRaw)
	if err != nil {
		return nil, fmt.Errorf("cannot parse certificates: %w", err)
	}
	sd.certificates = certs

	if !der.SkipOptionalASN1(cryptobyte_asn1.Tag(1).ContextSpecific().Constructed()) {
		return nil, errors.New("malformed crls")
	}

	var sisRaw cryptobyte.String
	if !der.ReadASN1(&sisRaw, cryptobyte_asn1.SET) {
		return nil, errors.New("malformed signerInfos")
	}
	for !sisRaw.Empty() {
		var siRaw cryptobyte.String
		if !sisRaw.ReadASN1Element(&siRaw, cryptobyte_asn1.SEQUENCE) {
			return nil, errors.New("malformed signerInfo")
		}
		si, err := readSignerInfo(siRaw)
		if err != nil {
			return nil, fmt.Errorf("cannot read signedInfo: %w", err)
		}
		sd.signerInfos = append(sd.signerInfos, *si)
	}

	return sd, nil
}

func unwrapSignedData(der *cryptobyte.String) error {
	s := *der
	if !s.ReadASN1(&s, cryptobyte_asn1.SEQUENCE) {
		return errors.New("malformed input")
	}
	if !s.PeekASN1Tag(cryptobyte_asn1.OBJECT_IDENTIFIER) {
		return nil
	}

	ci, err := readContentInfo(*der)
	if err != nil {
		return fmt.Errorf("cannot read contentInfo: %w", err)
	}
	if !ci.contentType.Equal(oidSignedData) {
		return errors.New("not signed data")
	}

	*der = cryptobyte.String(ci.content)
	return nil
}

type SignedData struct {
	Certificates []*x509.Certificate
	contentInfo  contentInfo
	signers      []issuerAndSerialNumber
}

func UnmarshalSignedData(data []byte) (*SignedData, error) {
	data, err := fixupDERLengths(data)
	if err != nil {
		return nil, err
	}

	der := cryptobyte.String(data)
	if err := unwrapSignedData(&der); err != nil {
		return nil, fmt.Errorf("cannot unwrap signedData: %w", err)
	}

	sd, err := readSignedData(der)
	if err != nil {
		return nil, fmt.Errorf("cannot read signedData: %w", err)
	}

	var signers []issuerAndSerialNumber
	for _, s := range sd.signerInfos {
		signers = append(signers, s.issuerAndSerialNumber)
	}

	return &SignedData{
		Certificates: sd.certificates,
		contentInfo:  sd.contentInfo,
		signers:      signers}, nil
}

func (p *SignedData) getCertFrom(ias *issuerAndSerialNumber) *x509.Certificate {
	for _, c := range p.Certificates {
		if c.SerialNumber.Cmp(&ias.serialNumber) == 0 && bytes.Equal(c.RawIssuer, ias.issuerNameRaw) {
			return c
		}
	}
	return nil
}

func (p *SignedData) GetSigners() []*x509.Certificate {
	var certs []*x509.Certificate

	for _, s := range p.signers {
		c := p.getCertFrom(&s)
		if c == nil {
			return nil
		}
		certs = append(certs, c)
	}

	return certs
}

func (p *SignedData) ContentType() asn1.ObjectIdentifier {
	return p.contentInfo.contentType
}

func (p *SignedData) Content() []byte {
	return p.contentInfo.content
}
