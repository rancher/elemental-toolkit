// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"github.com/canonical/go-efilib/internal/uefi"
)

var (
	HashAlgorithmSHA1Guid   = GUID(uefi.EFI_HASH_ALGORITHM_SHA1_GUID)
	HashAlgorithmSHA256Guid = GUID(uefi.EFI_HASH_ALGORITHM_SHA256_GUID)
	HashAlgorithmSHA224Guid = GUID(uefi.EFI_HASH_ALGORITHM_SHA224_GUID)
	HashAlgorithmSHA384Guid = GUID(uefi.EFI_HASH_ALGORITHM_SHA384_GUID)
	HashAlgorithmSHA512Guid = GUID(uefi.EFI_HASH_ALGORITHM_SHA512_GUID)

	CertTypeRSA2048SHA256Guid = GUID(uefi.EFI_CERT_TYPE_RSA2048_SHA256_GUID)
	CertTypePKCS7Guid         = GUID(uefi.EFI_CERT_TYPE_PKCS7_GUID)

	CertSHA1Guid   = GUID(uefi.EFI_CERT_SHA1_GUID)
	CertSHA256Guid = GUID(uefi.EFI_CERT_SHA256_GUID)
	CertSHA224Guid = GUID(uefi.EFI_CERT_SHA224_GUID)
	CertSHA384Guid = GUID(uefi.EFI_CERT_SHA384_GUID)
	CertSHA512Guid = GUID(uefi.EFI_CERT_SHA512_GUID)

	CertRSA2048Guid       = GUID(uefi.EFI_CERT_RSA2048_GUID)
	CertRSA2048SHA1Guid   = GUID(uefi.EFI_CERT_RSA2048_SHA1_GUID)
	CertRSA2048SHA256Guid = GUID(uefi.EFI_CERT_RSA2048_SHA256_GUID)

	CertX509Guid       = GUID(uefi.EFI_CERT_X509_GUID)
	CertX509SHA256Guid = GUID(uefi.EFI_CERT_X509_SHA256_GUID)
	CertX509SHA384Guid = GUID(uefi.EFI_CERT_X509_SHA384_GUID)
	CertX509SHA512Guid = GUID(uefi.EFI_CERT_X509_SHA512_GUID)

	GlobalVariable            = GUID(uefi.EFI_GLOBAL_VARIABLE)
	ImageSecurityDatabaseGuid = GUID(uefi.EFI_IMAGE_SECURITY_DATABASE_GUID)
)
