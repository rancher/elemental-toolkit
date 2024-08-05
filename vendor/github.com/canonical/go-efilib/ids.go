// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"github.com/canonical/go-efilib/internal/uefi"
)

var (
	// HashAlgorithmSHA1Guid corresponds to the SHA1 algorithm.
	HashAlgorithmSHA1Guid GUID = GUID(uefi.EFI_HASH_ALGORITHM_SHA1_GUID)

	// HashAlgorithmSHA256Guid corresponds to the SHA256 algorithm.
	HashAlgorithmSHA256Guid GUID = GUID(uefi.EFI_HASH_ALGORITHM_SHA256_GUID)

	// HashAlgorithmSHA224Guid corresponds to the SHA224 algorithm.
	HashAlgorithmSHA224Guid GUID = GUID(uefi.EFI_HASH_ALGORITHM_SHA224_GUID)

	// HashAlgorithmSHA384Guid corresponds to the SHA384 algorithm.
	HashAlgorithmSHA384Guid GUID = GUID(uefi.EFI_HASH_ALGORITHM_SHA384_GUID)

	// HashAlgorithmSHA412Guid corresponds to the SHA512 algorithm.
	HashAlgorithmSHA512Guid GUID = GUID(uefi.EFI_HASH_ALGORITHM_SHA512_GUID)

	// CertTypeRSA2048SHA256Guid is used to define the type of a
	// WinCertificateGUID that corresponds to a PKCS#1-v1.5 encoded RSA2048
	// SHA256 signature and is implemented by the *WinCertificateGUIDPKCS1v15
	// type.
	CertTypeRSA2048SHA256Guid GUID = GUID(uefi.EFI_CERT_TYPE_RSA2048_SHA256_GUID)

	// CertTypePKCS7Guid is used to define the type of a WinCertificateGUID
	// that corresponds to a detached PKCS#7 signature and is implemented by
	// the *WinCertificatePKCS7 type.
	CertTypePKCS7Guid GUID = GUID(uefi.EFI_CERT_TYPE_PKCS7_GUID)

	// CertSHA1Guid is used to define the type of a signature list that
	// contains a SHA1 digest.
	CertSHA1Guid GUID = GUID(uefi.EFI_CERT_SHA1_GUID)

	// CertSHA256Guid is used to define the type of a signature list that
	// contains a SHA-256 digest.
	CertSHA256Guid GUID = GUID(uefi.EFI_CERT_SHA256_GUID)

	// CertSHA224Guid is used to define the type of a signature list that
	// contains a SHA-224 digest.
	CertSHA224Guid GUID = GUID(uefi.EFI_CERT_SHA224_GUID)

	// CertSHA384Guid is used to define the type of a signature list that
	// contains a SHA-384 digest.
	CertSHA384Guid GUID = GUID(uefi.EFI_CERT_SHA384_GUID)

	// CertSHA512Guid is used to define the type of a signature list that
	// contains a SHA-512 digest.
	CertSHA512Guid GUID = GUID(uefi.EFI_CERT_SHA512_GUID)

	// CertRSA2048Guid is used to define the type of a signature list that
	// contains RSA2048 public keys.
	CertRSA2048Guid GUID = GUID(uefi.EFI_CERT_RSA2048_GUID)

	// CertRSA2048SHA1Guid is used to define the type of a signature list
	// that contains SHA1 digests of RSA2048 public keys.
	CertRSA2048SHA1Guid GUID = GUID(uefi.EFI_CERT_RSA2048_SHA1_GUID)

	// CertRSA2048SHA256Guid is used to define the type of a signature list
	// that contains SHA-256 digesta of RSA2048 public keya.
	CertRSA2048SHA256Guid GUID = GUID(uefi.EFI_CERT_RSA2048_SHA256_GUID)

	// CertX509Guid is used to define the type of a signature list that
	// contains a DER encoded X.509 certificate.
	CertX509Guid GUID = GUID(uefi.EFI_CERT_X509_GUID)

	// CertX509SHA256Guid is used to define the type of a signature list
	// that contains the SHA-256 digest of the TBS content of a X.509
	// certificate, and a revocation time.
	CertX509SHA256Guid GUID = GUID(uefi.EFI_CERT_X509_SHA256_GUID)

	// CertX509SHA384Guid is used to define the type of a signature list
	// that contains the SHA-384 digest of the TBS content of a X.509
	// certificate, and a revocation time.
	CertX509SHA384Guid GUID = GUID(uefi.EFI_CERT_X509_SHA384_GUID)

	// CertX509SHA512Guid is used to define the type of a signature list
	// that contains the SHA-512 digest of the TBS content of a X.509
	// certificate, and a revocation time.
	CertX509SHA512Guid GUID = GUID(uefi.EFI_CERT_X509_SHA512_GUID)

	// GlobalVariable is the namespace for many variables within the UEFI specification.
	GlobalVariable GUID = GUID(uefi.EFI_GLOBAL_VARIABLE)

	// ImageSecurityDatabaseGuid is the namespace for signature databases defined
	// byt the UEFI specification.
	ImageSecurityDatabaseGuid GUID = GUID(uefi.EFI_IMAGE_SECURITY_DATABASE_GUID)
)
