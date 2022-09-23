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

package tcg

import (
	"encoding/asn1"

	"github.com/canonical/go-tpm2"
)

const (
	// Handle for RSA2048 EK certificate, see section 7.8 of "TCG TPM v2.0 Provisioning Guidance" Version 1.0, Revision 1.0, 15 March 2017.
	EKCertHandle tpm2.Handle = 0x01c00002

	// Default RSA2048 SRK handle, see section 7.8 of "TCG TPM v2.0 Provisioning Guidance" Version 1.0, Revision 1.0, 15 March 2017
	SRKHandle tpm2.Handle = 0x81000001

	// Default RSA2048 EK handle, see section 7.8 of "TCG TPM v2.0 Provisioning Guidance" Version 1.0, Revision 1.0, 15 March 2017
	EKHandle tpm2.Handle = 0x81010001

	SANDirectoryNameTag = 4 // Subject Alternative Name directoryName, see section 4.2.16 or RFC5280
)

func MakeDefaultSRKTemplate() *tpm2.Public {
	return &tpm2.Public{
		Type:    tpm2.ObjectTypeRSA,
		NameAlg: tpm2.HashAlgorithmSHA256,
		Attrs: tpm2.AttrFixedTPM | tpm2.AttrFixedParent | tpm2.AttrSensitiveDataOrigin | tpm2.AttrUserWithAuth | tpm2.AttrNoDA |
			tpm2.AttrRestricted | tpm2.AttrDecrypt,
		Params: &tpm2.PublicParamsU{
			RSADetail: &tpm2.RSAParams{
				Symmetric: tpm2.SymDefObject{
					Algorithm: tpm2.SymObjectAlgorithmAES,
					KeyBits:   &tpm2.SymKeyBitsU{Sym: 128},
					Mode:      &tpm2.SymModeU{Sym: tpm2.SymModeCFB}},
				Scheme:   tpm2.RSAScheme{Scheme: tpm2.RSASchemeNull},
				KeyBits:  2048,
				Exponent: 0}},
		Unique: &tpm2.PublicIDU{RSA: make(tpm2.PublicKeyRSA, 256)}}
}

func MakeDefaultEKTemplate() *tpm2.Public {
	return &tpm2.Public{
		Type:    tpm2.ObjectTypeRSA,
		NameAlg: tpm2.HashAlgorithmSHA256,
		Attrs: tpm2.AttrFixedTPM | tpm2.AttrFixedParent | tpm2.AttrSensitiveDataOrigin | tpm2.AttrAdminWithPolicy | tpm2.AttrRestricted |
			tpm2.AttrDecrypt,
		AuthPolicy: []byte{0x83, 0x71, 0x97, 0x67, 0x44, 0x84, 0xb3, 0xf8, 0x1a, 0x90, 0xcc, 0x8d, 0x46, 0xa5, 0xd7, 0x24, 0xfd, 0x52, 0xd7,
			0x6e, 0x06, 0x52, 0x0b, 0x64, 0xf2, 0xa1, 0xda, 0x1b, 0x33, 0x14, 0x69, 0xaa},
		Params: &tpm2.PublicParamsU{
			RSADetail: &tpm2.RSAParams{
				Symmetric: tpm2.SymDefObject{
					Algorithm: tpm2.SymObjectAlgorithmAES,
					KeyBits:   &tpm2.SymKeyBitsU{Sym: 128},
					Mode:      &tpm2.SymModeU{Sym: tpm2.SymModeCFB}},
				Scheme:   tpm2.RSAScheme{Scheme: tpm2.RSASchemeNull},
				KeyBits:  2048,
				Exponent: 0}},
		Unique: &tpm2.PublicIDU{RSA: make(tpm2.PublicKeyRSA, 256)}}
}

var (
	// srkTemplate is the default RSA2048 SRK template, see section 7.5.1 of "TCG TPM v2.0 Provisioning Guidance", version 1.0, revision 1.0, 15 March 2017.
	SRKTemplate = MakeDefaultSRKTemplate()

	// Default RSA2048 EK template, see section B.3.3 of "TCG EK Credential Profile For TPM Family 2.0; Level 0", Version 2.1, Revision 13, 10 December 2018
	EKTemplate = MakeDefaultEKTemplate()

	OIDExtensionSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17} // id-ce-subjectAltName, see section 4.2.16 of RFC5280

	// TCG specific OIDs, see section 4 of "TCG EK Credential Profile For TPM Family 2.0; Level 0", Version 2.1, Revision 13, 10 December 2018.
	OIDTcgAttributeTpmManufacturer = asn1.ObjectIdentifier{2, 23, 133, 2, 1} // tcg-at-tpmManufacturer
	OIDTcgAttributeTpmModel        = asn1.ObjectIdentifier{2, 23, 133, 2, 2} // tcg-at-tpmModel
	OIDTcgAttributeTpmVersion      = asn1.ObjectIdentifier{2, 23, 133, 2, 3} // tcg-at-tpmVersion
	OIDTcgKpEkCertificate          = asn1.ObjectIdentifier{2, 23, 133, 8, 1} // tcg-kp-EKCertificate
)
