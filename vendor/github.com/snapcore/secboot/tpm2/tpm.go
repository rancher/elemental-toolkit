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
	"crypto"
	"crypto/rsa"
	_ "crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"
	"github.com/snapcore/snapd/httputil"
	"github.com/snapcore/snapd/osutil"
	"github.com/snapcore/snapd/osutil/sys"

	"golang.org/x/xerrors"

	"github.com/snapcore/secboot/internal/tcg"
	"github.com/snapcore/secboot/internal/tcti"
	"github.com/snapcore/secboot/internal/truststore"
)

// DeviceAttributes contains details about the TPM extracted from a manufacturer issued endorsement key certificate.
type DeviceAttributes struct {
	Manufacturer    tpm2.TPMManufacturer
	Model           string
	FirmwareVersion uint32
}

// Connection corresponds to a connection to a TPM device, and is a wrapper around *tpm2.TPMContext.
type Connection struct {
	*tpm2.TPMContext
	verifiedEkCertChain      []*x509.Certificate
	verifiedDeviceAttributes *DeviceAttributes
	ek                       tpm2.ResourceContext
	provisionedSrk           tpm2.ResourceContext
	hmacSession              tpm2.SessionContext
}

// IsEnabled indicates whether the TPM is enabled or whether it has been disabled by the platform firmware. A TPM device can be
// disabled by the platform firmware by disabling the storage and endorsement hierarchies, but still remain visible to the operating
// system.
func (t *Connection) IsEnabled() bool {
	props, err := t.GetCapabilityTPMProperties(tpm2.PropertyStartupClear, 1, t.HmacSession().IncludeAttrs(tpm2.AttrAudit))
	if err != nil || len(props) == 0 {
		return false
	}
	const enabledMask = tpm2.AttrShEnable | tpm2.AttrEhEnable
	return tpm2.StartupClearAttributes(props[0].Value)&enabledMask == enabledMask
}

// VerifiedEKCertChain returns the verified certificate chain for the endorsement key certificate obtained from this TPM. It was
// verified using one of the built-in TPM manufacturer root CA certificates.
func (t *Connection) VerifiedEKCertChain() []*x509.Certificate {
	return t.verifiedEkCertChain
}

// VerifiedDeviceAttributes returns the TPM device attributes for this TPM, obtained from the verified endorsement key certificate.
func (t *Connection) VerifiedDeviceAttributes() *DeviceAttributes {
	return t.verifiedDeviceAttributes
}

// EndorsementKey returns a reference to the TPM's persistent endorsement key, if one exists. If the endorsement key certificate has
// been verified, the returned ResourceContext will correspond to the object for which the certificate was issued and can safely be
// used to share secrets with the TPM.
func (t *Connection) EndorsementKey() (tpm2.ResourceContext, error) {
	if t.ek == nil {
		return nil, ErrTPMProvisioning
	}
	return t.ek, nil
}

// HmacSession returns a HMAC session instance which was created in order to conduct a proof-of-ownership check of the private part
// of the endorsement key on the TPM. It is retained in order to reduce the number of sessions that need to be created during unseal
// operations, and is created with a symmetric algorithm so that it is suitable for parameter encryption.
// If the connection was created with SecureConnectToDefaultTPM, the session is salted with a value protected by the public part
// of the key associated with the verified endorsement key certificate. The session key can only be retrieved by and used on the TPM
// for which the endorsement certificate was issued. If the connection was created with ConnectToDefaultTPM, the session may be
// salted with a value protected by the public part of the endorsement key if one exists or one is able to be created, but as the key
// is not associated with a verified credential, there is no guarantee that only the TPM is able to retrieve the session key.
func (t *Connection) HmacSession() tpm2.SessionContext {
	if t.hmacSession == nil {
		return nil
	}
	return t.hmacSession.WithAttrs(tpm2.AttrContinueSession)
}

func (t *Connection) Close() error {
	t.FlushContext(t.hmacSession)
	return t.TPMContext.Close()
}

// createTransientEk creates a new primary key in the endorsement hierarchy using the default RSA2048 EK template.
func createTransientEk(tpm *tpm2.TPMContext) (tpm2.ResourceContext, error) {
	session, err := tpm.StartAuthSession(nil, tpm.EndorsementHandleContext(), tpm2.SessionTypeHMAC, nil, tpm2.HashAlgorithmSHA256)
	if err != nil {
		return nil, xerrors.Errorf("cannot start auth session: %w", err)
	}
	defer tpm.FlushContext(session)

	ek, _, _, _, _, err := tpm.CreatePrimary(tpm.EndorsementHandleContext(), nil, tcg.EKTemplate, nil, nil, session)
	return ek, err
}

// verifyEk verifies that the public area of the ResourceContext that was read back from the TPM is associated with the supplied EK
// certificate. It does this by obtaining the public key from the EK certificate, inserting it in to the standard EK template,
// computing the expected name of the EK object and then verifying that this name matches the result of ResourceContext.Name. This
// works because go-tpm2 cross-checks that the name and public area returned from TPM2_ReadPublic match when initializing the
// ResourceContext.
//
// Success confirms that the ResourceContext references the public area associated with the public key of the supplied EK certificate.
// If that certificate has been verified, the ResourceContext can safely be used to encrypt secrets that can only be decrpyted and
// used by the TPM for which the EK certificate was issued, eg, for salting an authorization session that is then used for parameter
// encryption.
func verifyEk(cert *x509.Certificate, ek tpm2.ResourceContext) error {
	// Obtain the RSA public key from the endorsement certificate
	pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("cannot obtain RSA public key from certificate")
	}

	// Insert the RSA public key in to the EK template to compute the name of the EK object we expected to read back from the TPM.
	var ekPublic *tpm2.Public
	b, _ := mu.MarshalToBytes(tcg.EKTemplate)
	mu.UnmarshalFromBytes(b, &ekPublic)

	// The default exponent of 2^^16-1 is indicated by the value of 0 in the public area.
	if pubKey.E != 65537 {
		ekPublic.Params.RSADetail.Exponent = uint32(pubKey.E)
	}
	ekPublic.Unique.RSA = pubKey.N.Bytes()

	expectedEkName, err := ekPublic.Name()
	if err != nil {
		panic(fmt.Sprintf("cannot compute expected name of EK object: %v", err))
	}

	// Verify that the public area associated with context corresponds to the object that the endorsement certificate was issued for.
	// We do this by comparing the name read back from the TPM with the one we computed from the EK template with the certificate's
	// public key inserted in to it (remember that go-tpm2 has already verified that the name that was read back is consistent with the
	// public area).
	if !bytes.Equal(ek.Name(), expectedEkName) {
		// An exponent of 0 in the public area corresponds to the default (65537) exponent, but some TPM's don't return 0 in the
		// public area (my Nuvoton TPM, for example). If the initial name comparison with exponent == 0 failed, try exponent == 65537.
		ekPublic.Params.RSADetail.Exponent = uint32(pubKey.E)
		expectedEkName, err := ekPublic.Name()
		if err != nil {
			panic(fmt.Sprintf("cannot compute expected name of EK object: %v", err))
		}
		if !bytes.Equal(ek.Name(), expectedEkName) {
			return errors.New("public area doesn't match certificate")
		}
	}

	return nil
}

type verificationError struct {
	err error
}

func (e verificationError) Error() string {
	return e.err.Error()
}

func (t *Connection) init() error {
	// Allow init to be called more than once by flushing the previous session
	if t.hmacSession != nil && t.hmacSession.Handle() != tpm2.HandleUnassigned {
		t.FlushContext(t.hmacSession)
		t.hmacSession = nil
	}
	t.ek = nil
	t.provisionedSrk = nil

	secureMode := len(t.verifiedEkCertChain) > 0

	// Acquire an unverified ResourceContext for the EK. If there is no object at the persistent EK index, then attempt to create
	// a transient EK with the supplied authorization if this is a secure connection.
	//
	// Under the hood, go-tpm2 initializes the ResourceContext with TPM2_ReadPublic (or TPM2_CreatePrimary if we create a new one),
	// and it cross-checks that the returned name and public area match. The returned name is available via ek.Name and the
	// returned public area is retained by ek and used to share secrets with the TPM.
	//
	// Without verification against the EK certificate, ek isn't yet safe to use for secret sharing with the TPM.
	ek, err := func() (tpm2.ResourceContext, error) {
		ek, err := t.CreateResourceContextFromTPM(tcg.EKHandle)
		if err == nil || !secureMode {
			return ek, nil
		}
		if !tpm2.IsResourceUnavailableError(err, tcg.EKHandle) {
			return nil, err
		}
		if ek, err := createTransientEk(t.TPMContext); err == nil {
			return ek, nil
		}
		return nil, err
	}()
	if err != nil {
		// A lack of EK should be fatal in this context
		return xerrors.Errorf("cannot obtain context for EK: %w", err)
	}

	ekIsPersistent := func() bool {
		return ek != nil && ek.Handle() == tcg.EKHandle
	}

	defer func() {
		if ek == nil || ekIsPersistent() {
			return
		}
		t.FlushContext(ek)
	}()

	if secureMode {
		// Verify that ek is associated with the verified EK certificate. If the first attempt fails and ek references a persistent
		// object, then try to create a transient EK with the provided authorization and make another attempt at verification, in case
		// the persistent object isn't a valid EK.
		rc, err := func() (tpm2.ResourceContext, error) {
			err := verifyEk(t.verifiedEkCertChain[0], ek)
			if err == nil {
				return nil, nil
			}
			if ek.Handle() != tcg.EKHandle {
				// If this was already a transient EK, fail now
				return nil, err
			}
			transientEk, err2 := createTransientEk(t.TPMContext)
			if err2 != nil {
				return nil, err
			}
			err = verifyEk(t.verifiedEkCertChain[0], transientEk)
			if err == nil {
				return transientEk, nil
			}
			return nil, err
		}()
		if err != nil {
			return verificationError{xerrors.Errorf("cannot verify public area of endorsement key read from the TPM: %w", err)}
		}
		if rc != nil {
			// The persistent EK was bad, and we created and verified a transient EK instead
			ek = rc
		}
	} else if ek != nil {
		// If we don't have a verified EK certificate and ek is a persistent object, just do a sanity check that the public area returned
		// from the TPM has the expected properties. If it doesn't, then don't use it, as TPM2_StartAuthSession might fail.
		if ok, err := isObjectPrimaryKeyWithTemplate(t.TPMContext, t.EndorsementHandleContext(), ek, tcg.EKTemplate, nil); err != nil {
			return xerrors.Errorf("cannot determine if object is a primary key in the endorsement hierarchy: %w", err)
		} else if !ok {
			ek = nil
		}
	}

	// Verify that the TPM we're connected to is the one that the endorsement certificate was issued for (if we've validated it) by
	// creating a session that's salted with a value protected by the public part of the endorsement key, using that to integrity protect
	// a command and verifying we get a valid response. The salt (and therefore the session key) can only be recovered on and used by the
	// TPM for which the endorsement certificate was issued, so a correct response means we're communicating with that TPM.
	symmetric := tpm2.SymDef{
		Algorithm: tpm2.SymAlgorithmAES,
		KeyBits:   &tpm2.SymKeyBitsU{Sym: 128},
		Mode:      &tpm2.SymModeU{Sym: tpm2.SymModeCFB}}
	session, err := t.StartAuthSession(ek, nil, tpm2.SessionTypeHMAC, &symmetric, defaultSessionHashAlgorithm, nil)
	if err != nil {
		return xerrors.Errorf("cannot create HMAC session: %w", err)
	}
	succeeded := false
	defer func() {
		if succeeded {
			return
		}
		t.FlushContext(session)
	}()

	if secureMode {
		_, err = t.GetRandom(20, session.WithAttrs(tpm2.AttrContinueSession|tpm2.AttrAudit))
		if err != nil {
			if isAuthFailError(err, tpm2.CommandGetRandom, 1) {
				return verificationError{errors.New("endorsement key proof of ownership check failed")}
			}
			return xerrors.Errorf("cannot execute command to complete EK proof of ownership check: %w", err)
		}
	}

	succeeded = true

	if ekIsPersistent() {
		t.ek = ek
	}
	t.hmacSession = session
	return nil
}

// readEkCertFromTPM reads the manufacturer injected certificate for the default RSA2048 EK from the standard index, and
// returns it as a DER encoded byte slice.
func readEkCertFromTPM(tpm *tpm2.TPMContext) ([]byte, error) {
	ekCertIndex, err := tpm.CreateResourceContextFromTPM(tcg.EKCertHandle)
	if err != nil {
		return nil, xerrors.Errorf("cannot create context: %w", err)
	}

	ekCertPub, _, err := tpm.NVReadPublic(ekCertIndex)
	if err != nil {
		return nil, xerrors.Errorf("cannot read public area of index: %w", err)
	}

	cert, err := tpm.NVRead(ekCertIndex, ekCertIndex, ekCertPub.Size, 0, nil)
	if err != nil {
		return nil, xerrors.Errorf("cannot read index: %w", err)
	}

	return cert, nil
}

// connectToDefaultTPM opens a connection to the default TPM device.
func connectToDefaultTPM() (*tpm2.TPMContext, error) {
	tcti, err := tcti.OpenDefault()
	if err != nil {
		if isPathError(err) {
			return nil, ErrNoTPM2Device
		}
		return nil, xerrors.Errorf("cannot open TPM device: %w", err)
	}

	tpm := tpm2.NewTPMContext(tcti)
	if !tpm.IsTPM2() {
		tpm.Close()
		return nil, ErrNoTPM2Device
	}

	return tpm, nil
}

func isExtKeyUsageAny(usage []x509.ExtKeyUsage) bool {
	for _, u := range usage {
		if u == x509.ExtKeyUsageAny {
			return true
		}
	}
	return false
}

func isExtKeyUsageEkCertificate(usage []asn1.ObjectIdentifier) bool {
	for _, u := range usage {
		if u.Equal(tcg.OIDTcgKpEkCertificate) {
			return true
		}
	}
	return false
}

// checkChainForEkCertUsage checks whether the specified certificate chain is valid for a TPM endorsement key certificate.
// For any certificate in the chain that defines extended key usage, that key usage must either be anyExtendedKeyUsage (RFC5280),
// or it must contain tcg-kp-EKCertificate.
func checkChainForEkCertUsage(chain []*x509.Certificate) bool {
	if len(chain) == 0 {
		return false
	}

	for i := len(chain) - 1; i >= 0; i-- {
		cert := chain[i]
		if len(cert.ExtKeyUsage) == 0 && len(cert.UnknownExtKeyUsage) == 0 {
			continue
		}

		if isExtKeyUsageAny(cert.ExtKeyUsage) {
			continue
		}

		if isExtKeyUsageEkCertificate(cert.UnknownExtKeyUsage) {
			continue
		}

		return false
	}

	return true
}

func parseDeviceAttributesFromDirectoryName(dirName pkix.RDNSequence) (*DeviceAttributes, pkix.RDNSequence, error) {
	var attrs DeviceAttributes
	var rdnsOut pkix.RelativeDistinguishedNameSET

	hasManufacturer, hasModel, hasVersion := false, false, false

	for _, rdns := range dirName {
		for _, atv := range rdns {
			switch {
			case atv.Type.Equal(tcg.OIDTcgAttributeTpmManufacturer):
				if hasManufacturer {
					return nil, nil, asn1.StructuralError{Msg: "duplicate TPM manufacturer"}
				}
				hasManufacturer = true
				s, ok := atv.Value.(string)
				if !ok {
					return nil, nil, asn1.StructuralError{Msg: "invalid TPM attribute value"}
				}
				if !strings.HasPrefix(s, "id:") {
					return nil, nil, asn1.StructuralError{Msg: "invalid TPM manufacturer"}
				}
				hex, err := hex.DecodeString(strings.TrimPrefix(s, "id:"))
				if err != nil {
					return nil, nil, asn1.StructuralError{Msg: fmt.Sprintf("invalid TPM manufacturer: %v", err)}
				}
				if len(hex) != 4 {
					return nil, nil, asn1.StructuralError{Msg: "invalid TPM manufacturer: too short"}
				}
				attrs.Manufacturer = tpm2.TPMManufacturer(binary.BigEndian.Uint32(hex))
			case atv.Type.Equal(tcg.OIDTcgAttributeTpmModel):
				if hasModel {
					return nil, nil, asn1.StructuralError{Msg: "duplicate TPM model"}
				}
				hasModel = true
				s, ok := atv.Value.(string)
				if !ok {
					return nil, nil, asn1.StructuralError{Msg: "invalid TPM attribute value"}
				}
				attrs.Model = s
			case atv.Type.Equal(tcg.OIDTcgAttributeTpmVersion):
				if hasVersion {
					return nil, nil, asn1.StructuralError{Msg: "duplicate TPM firmware version"}
				}
				hasVersion = true
				s, ok := atv.Value.(string)
				if !ok {
					return nil, nil, asn1.StructuralError{Msg: "invalid TPM attribute value"}
				}
				if !strings.HasPrefix(s, "id:") {
					return nil, nil, asn1.StructuralError{Msg: "invalid TPM firmware version"}
				}
				hex, err := hex.DecodeString(strings.TrimPrefix(s, "id:"))
				if err != nil {
					return nil, nil, asn1.StructuralError{Msg: fmt.Sprintf("invalid TPM firmware version: %v", err)}
				}
				b := make([]byte, 4)
				copy(b[len(b)-len(hex):], hex)
				attrs.FirmwareVersion = binary.BigEndian.Uint32(b)
			default:
				continue
			}
			rdnsOut = append(rdnsOut, atv)
		}
	}

	if hasManufacturer && hasModel && hasVersion {
		return &attrs, pkix.RDNSequence{rdnsOut}, nil
	}
	return nil, nil, errors.New("incomplete or missing attributes")
}

func parseDeviceAttributesFromSAN(data []byte) (*DeviceAttributes, pkix.RDNSequence, error) {
	var seq asn1.RawValue
	if rest, err := asn1.Unmarshal(data, &seq); err != nil {
		return nil, nil, err
	} else if len(rest) > 0 {
		return nil, nil, errors.New("trailing bytes after SAN extension")
	}
	if !seq.IsCompound || seq.Tag != asn1.TagSequence || seq.Class != asn1.ClassUniversal {
		return nil, nil, asn1.StructuralError{Msg: "invalid SAN sequence"}
	}

	rest := seq.Bytes
	for len(rest) > 0 {
		var err error
		var v asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &v)
		if err != nil {
			return nil, nil, err
		}

		if v.Class != asn1.ClassContextSpecific {
			return nil, nil, asn1.StructuralError{Msg: "invalid SAN entry"}
		}

		if v.Tag == tcg.SANDirectoryNameTag {
			var dirName pkix.RDNSequence
			if rest, err := asn1.Unmarshal(v.Bytes, &dirName); err != nil {
				return nil, nil, err
			} else if len(rest) > 0 {
				return nil, nil, errors.New("trailing bytes after SAN extension directory name")
			}

			return parseDeviceAttributesFromDirectoryName(dirName)
		}
	}

	return nil, nil, errors.New("no directoryName")
}

// isCertificateTrustedCA determines whether the supplied certificate is one of the trusted root CAs by comparing a digest of it
// with the built-in digests.
func isCertificateTrustedCA(cert *x509.Certificate) bool {
	h := crypto.SHA256.New()
	h.Write(cert.Raw)
	hash := h.Sum(nil)

Outer:
	for _, rootHash := range truststore.RootCAHashes {
		for i, b := range rootHash {
			if b != hash[i] {
				continue Outer
			}
		}
		return true
	}
	return false
}

type ekCertData struct {
	Cert    []byte
	Parents [][]byte
}

// verifyEkCertificate verifies the provided certificate and intermediate certificates against the built-in roots, and verifies
// that the certificate is a valid EK certificate, according to the "TCG EK Credential Profile" specification.
//
// On success, it returns a verified certificate chain. This function will also return success if there is no certificate and
// it is executed inside a guest VM, in order to support fallback to a non-secure connection when using swtpm in a guest VM.
func verifyEkCertificate(data *ekCertData) ([]*x509.Certificate, *DeviceAttributes, error) {
	// Parse EK cert
	cert, err := x509.ParseCertificate(data.Cert)
	if err != nil {
		return nil, nil, xerrors.Errorf("cannot parse endorsement key certificate: %w", err)
	}

	// Parse other certs, building root and intermediates store
	roots := x509.NewCertPool()
	intermediates := x509.NewCertPool()
	for _, d := range data.Parents {
		c, err := x509.ParseCertificate(d)
		if err != nil {
			return nil, nil, xerrors.Errorf("cannot parse certificate: %w", err)
		}
		if isCertificateTrustedCA(c) {
			roots.AddCert(c)
		} else {
			intermediates.AddCert(c)
		}
	}

	if cert.PublicKeyAlgorithm != x509.RSA {
		return nil, nil, errors.New("certificate contains a public key with the wrong algorithm")
	}

	// MUST have valid basic constraints with CA=FALSE
	if cert.IsCA || !cert.BasicConstraintsValid {
		return nil, nil, errors.New("certificate contains invalid basic constraints")
	}

	var attrs *DeviceAttributes
	for _, e := range cert.Extensions {
		if e.Id.Equal(tcg.OIDExtensionSubjectAltName) {
			// SubjectAltName MUST be critical if subject is empty
			if len(cert.Subject.Names) == 0 && !e.Critical {
				return nil, nil, errors.New("certificate with empty subject contains non-critical SAN extension")
			}
			var err error
			var attrsRDN pkix.RDNSequence
			attrs, attrsRDN, err = parseDeviceAttributesFromSAN(e.Value)
			// SubjectAltName MUST include TPM manufacturer, model and firmware version
			if err != nil {
				return nil, nil, xerrors.Errorf("cannot parse TPM device attributes: %w", err)
			}
			if len(cert.Subject.Names) == 0 {
				// If subject is empty, fill the Subject field with the TPM device attributes so that String() returns something useful
				cert.Subject.FillFromRDNSequence(&attrsRDN)
				cert.Subject.ExtraNames = cert.Subject.Names
			}
			break
		}
	}

	// SubjectAltName MUST exist. If it does exist but doesn't contain the correct TPM device attributes, we would have returned earlier.
	if attrs == nil {
		return nil, nil, errors.New("certificate has no SAN extension")
	}

	// If SAN contains only fields unhandled by crypto/x509 and it is marked as critical, then it ends up here. Remove it because
	// we've handled it ourselves and x509.Certificate.Verify fails if we leave it here.
	for i, e := range cert.UnhandledCriticalExtensions {
		if e.Equal(tcg.OIDExtensionSubjectAltName) {
			copy(cert.UnhandledCriticalExtensions[i:], cert.UnhandledCriticalExtensions[i+1:])
			cert.UnhandledCriticalExtensions = cert.UnhandledCriticalExtensions[:len(cert.UnhandledCriticalExtensions)-1]
			break
		}
	}

	// Key Usage MUST contain keyEncipherment
	if cert.KeyUsage&x509.KeyUsageKeyEncipherment == 0 {
		return nil, nil, errors.New("certificate has incorrect key usage")
	}

	// Verify EK certificate for any usage - we're going to verify the Extended Key Usage afterwards
	opts := x509.VerifyOptions{
		Intermediates: intermediates,
		Roots:         roots,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}
	candidates, err := cert.Verify(opts)
	if err != nil {
		return nil, nil, xerrors.Errorf("certificate verification failed: %w", err)
	}

	// Extended Key Usage MUST contain tcg-kp-EKCertificate (and also require that the usage is nested)
	var chain []*x509.Certificate
	for _, c := range candidates {
		if checkChainForEkCertUsage(c) {
			chain = c
			break
		}
	}

	if chain == nil {
		return nil, nil, errors.New("no certificate chain has the correct extended key usage")
	}

	// At this point, we've verified that the endorsent certificate has the correct properties and was issued by a trusted TPM
	// manufacturer, and is therefore a valid assertion by that manufacturer that the contained public key is associated with a
	// properly formed endorsement key with the expected properties (restricted, non-duplicable decrypt key), generated from a
	// private seed injected in to a genuine TPM by them. Secrets encrypted by this public key can only be decrypted by and used
	// on the TPM for which this certificate was issued.

	return chain, attrs, nil
}

// fetchParentCertificates attempts to fetch all of the parent certificates for the provided leaf certificate. This requires that the
// leaf certificate and any intermediate certificates support the Authority Information Access extension in order to obtain a complete
// certificate chain. It will stop when it encounters a self-signed certificate, or a certificate that doesn't support the AIA
// extension.
func fetchParentCertificates(cert *x509.Certificate) ([][]byte, error) {
	client := httputil.NewHTTPClient(&httputil.ClientOptions{Timeout: 10 * time.Second})
	var out [][]byte

	for {
		if bytes.Equal(cert.RawIssuer, cert.RawSubject) {
			break
		}

		if len(cert.IssuingCertificateURL) == 0 {
			break
		}

		var parent *x509.Certificate
		var err error
		for _, issuerUrl := range cert.IssuingCertificateURL {
			if p, e := func(url string) (*x509.Certificate, error) {
				resp, err := client.Get(url)
				if err != nil {
					return nil, xerrors.Errorf("GET request failed: %w", err)
				}
				body, err := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					return nil, xerrors.Errorf("cannot read body: %w", err)
				}
				cert, err := x509.ParseCertificate(body)
				if err != nil {
					return nil, xerrors.Errorf("cannot parse certificate: %w", err)
				}
				return cert, nil
			}(issuerUrl); e == nil {
				parent = p
				err = nil
				break
			} else {
				err = xerrors.Errorf("download from %s failed: %w", issuerUrl, e)
			}
		}

		if err != nil {
			return nil, xerrors.Errorf("cannot download parent certificate of %v: %w", cert.Subject, err)
		}

		out = append(out, parent.Raw)
		cert = parent
	}

	return out, nil
}

// fetchEkCertificateChain will attempt to obtain the entire EK certificate chain, first by reading the EK certificate from
// the TPM and then fetching all of the parent certificates. If parentsOnly is true, the returned data will not include the
// actual EK certificate.
func fetchEkCertificateChain(tpm *tpm2.TPMContext, parentsOnly bool) (*ekCertData, error) {
	var data ekCertData

	if cert, err := readEkCertFromTPM(tpm); err != nil {
		return nil, xerrors.Errorf("cannot obtain endorsement key certificate from TPM: %w", err)
	} else {
		if !parentsOnly {
			data.Cert = cert
		}

		c, err := x509.ParseCertificate(cert)
		if err != nil {
			return nil, xerrors.Errorf("cannot parse endorsement key certificate: %w", err)
		}

		parents, err := fetchParentCertificates(c)
		if err != nil {
			return nil, xerrors.Errorf("cannot obtain parent certificates for %s: %w", c.Subject, err)
		}
		data.Parents = parents
	}

	return &data, nil
}

// saveEkCertificateChain will save the supplied EK certificate chain to the file at the specified path. The file will be updated
// atomically.
func saveEkCertificateChain(data *ekCertData, dest string) error {
	f, err := osutil.NewAtomicFile(dest, 0600, 0, sys.UserID(osutil.NoChown), sys.GroupID(osutil.NoChown))
	if err != nil {
		return xerrors.Errorf("cannot create new atomic file: %w", err)
	}
	defer f.Cancel()

	if _, err := mu.MarshalToWriter(f, data); err != nil {
		return xerrors.Errorf("cannot marshal cert chain: %w", err)
	}

	if err := f.Commit(); err != nil {
		return xerrors.Errorf("cannot atomically replace file: %w", err)
	}

	return nil
}

// FetchAndSaveEKCertificateChain attempts to obtain the endorsement key certificate for the TPM associated with the tpm parameter,
// download the parent certificates and then save them atomically to the specified file in a form that can be decoded by
// SecureConnectToDefaultTPM. This function requires network access.
//
// This depends on the presence of the Authority Information Access extension in the leaf certificate and any intermediate certificates,
// which must contain a URL for the parent certificate.
//
// This function will stop when it encounters a certificate that doesn't specify an issuer URL, or when it encounters a self-signed
// certificate.
//
// If no endorsement key certificate can be obtained, an error will be returned.
//
// If parentsOnly is true, this function will only save the parent certificates as long as the endorsement key certificate can be
// reliably obtained from the TPM.
func FetchAndSaveEKCertificateChain(tpm *Connection, parentsOnly bool, destPath string) error {
	data, err := fetchEkCertificateChain(tpm.TPMContext, parentsOnly)
	if err != nil {
		return err
	}

	return saveEkCertificateChain(data, destPath)
}

// SaveEKCertificateChain will save the specified EK certificate and associated parent certificates atomically to the specified file
// in a form that can be decoded by SecureConnectToDefaultTPM. This is useful in scenarios where the EK certificate cannot be located
// automatically, or the EK certificate or any intermediate certificates lack the Authority Information Access extension but the
// certificates have been downloaded manually by the caller.
//
// If the EK certificate can be obtained reliably from the TPM during establishment of a connection, then it can be omitted in order
// to save a file that only contains parent certificates. In this case, SecureConnectToDefaultTPM will attempt to obtain the EK
// certificate from the TPM and verify it against the supplied parent certificates.
func SaveEKCertificateChain(ekCert *x509.Certificate, parents []*x509.Certificate, destPath string) error {
	var data ekCertData
	if ekCert != nil {
		data.Cert = ekCert.Raw
	}
	for _, c := range parents {
		data.Parents = append(data.Parents, c.Raw)
	}

	return saveEkCertificateChain(&data, destPath)
}

// EncodeEKCertificateChain will write the specified EK certificate and associated parent certificates to the specified io.Writer in a
// form that can be decoded by SecureConnectToDefaultTPM. This is useful in scenarios where the EK certificate cannot be located
// automatically, or the EK certificate or any intermediate certificates lack the Authority Information Access extension but the
// certificates have been downloaded manually by the caller.
//
// If the EK certificate can be obtained reliably from the TPM during establishment of a connection, then it can be omitted in order
// to save a file that only contains parent certificates. In this case, SecureConnectToDefaultTPM will attempt to obtain the EK
// certificate from the TPM and verify it against the supplied parent certificates.
func EncodeEKCertificateChain(ekCert *x509.Certificate, parents []*x509.Certificate, w io.Writer) error {
	var data ekCertData
	if ekCert != nil {
		data.Cert = ekCert.Raw
	}
	for _, c := range parents {
		data.Parents = append(data.Parents, c.Raw)
	}

	if _, err := mu.MarshalToWriter(w, &data); err != nil {
		return xerrors.Errorf("cannot marshal cert chain: %w", err)
	}

	return nil
}

// ConnectToDefaultTPM will attempt to connect to the default TPM. It makes no attempt to verify the authenticity of the TPM. This
// function is useful for connecting to a device that isn't correctly provisioned and for which the endorsement hierarchy
// authorization value is unknown (so that it can be cleared), or for connecting to a device in order to execute
// FetchAndSaveEKCertificateChain. It should not be used in any other scenario.
//
// If no TPM2 device is available, then a ErrNoTPM2Device error will be returned.
func ConnectToDefaultTPM() (*Connection, error) {
	tpm, err := connectToDefaultTPM()
	if err != nil {
		return nil, err
	}

	t := &Connection{TPMContext: tpm}

	succeeded := false
	defer func() {
		if succeeded {
			return
		}
		t.Close()
	}()

	if err := t.init(); err != nil {
		var verifyErr verificationError
		if !tpm2.IsResourceUnavailableError(err, tpm2.AnyHandle) && !xerrors.As(err, &verifyErr) {
			return nil, xerrors.Errorf("cannot initialize TPM connection: %w", err)
		}
	}

	succeeded = true
	return t, nil
}

// SecureConnectToDefaultTPM will attempt to connect to the default TPM, verify the manufacturer issued endorsement key certificate
// against the built-in CA roots and then verify that the TPM is the one for which the endorsement certificate was issued.
//
// The ekCertDataReader argument should read from a file or buffer created previously by FetchAndSaveEKCertificateChain,
// SaveEKCertificateChain or EncodeEKCertificateChain. An error will be returned if this is not provided.
//
// If the data read from ekCertDataReader cannot be unmarshalled or parsed correctly, a EKCertVerificationError error will be
// returned.
//
// If ekCertDataReader does not contain an endorsement key certificate, this function will attempt to obtain the certificate for the
// TPM. This does not require network access. If this fails, a EKCertVerificationError error will be returned.
//
// If verification of the endorsement key certificate fails, a EKCertVerificationError error will be returned. This might mean that
// the data provided via ekCertDataReader is invalid and needs to be recreated.
//
// In order for the TPM to prove it is the device for which the endorsement key certificate was issued, an endorsement key is
// required. If the TPM doesn't contain a valid persistent endorsement key at the expected location (eg, if ProvisionTPM hasn't been
// executed yet), this function will attempt to create a transient endorsement key. This requires knowledge of the endorsement
// hierarchy authorization value, provided via the endorsementAuth argument, The endorsement hierarchy authorization value will be
// empty on a newly cleared device. If there is no valid persistent endorsement key and creation of a transient endorsement key fails,
// ErrTPMProvisioning will be returned. Note that creation of a transient endorsement key may take a long time on some TPMs (in excess
// of 10 seconds).
//
// If the TPM cannot prove it is the device for which the endorsement key certificate was issued, a TPMVerificationError error will be
// returned. This can happen if there is an object at the persistent endorsement key index but it is not the object for which the
// endorsement key certificate was issued, and creation of a transient endorsement key fails because the correct endorsement hierarchy
// authorization value hasn't been provided via the endorsementAuth argument.
//
// If no TPM2 device is available, then a ErrNoTPM2Device error will be returned.
func SecureConnectToDefaultTPM(ekCertDataReader io.Reader, endorsementAuth []byte) (*Connection, error) {
	if ekCertDataReader == nil {
		return nil, errors.New("no EK certificate data was provided")
	}

	tpm, err := connectToDefaultTPM()
	if err != nil {
		return nil, err
	}
	tpm.EndorsementHandleContext().SetAuthValue(endorsementAuth)

	succeeded := false
	defer func() {
		if succeeded {
			return
		}
		tpm.Close()
	}()

	t := &Connection{TPMContext: tpm}

	var certData *ekCertData
	// Unmarshal supplied EK cert data
	if _, err := mu.UnmarshalFromReader(ekCertDataReader, &certData); err != nil {
		return nil, EKCertVerificationError{fmt.Sprintf("cannot unmarshal supplied EK certificate data: %v", err)}
	}
	if len(certData.Cert) == 0 {
		// The supplied data only contains parent certificates. Retrieve the EK cert from the TPM.
		if cert, err := readEkCertFromTPM(tpm); err != nil {
			return nil, EKCertVerificationError{fmt.Sprintf("cannot obtain endorsement key certificate from TPM: %v", err)}
		} else {
			certData.Cert = cert
		}
	}

	chain, attrs, err := verifyEkCertificate(certData)
	if err != nil {
		return nil, EKCertVerificationError{err.Error()}
	}

	t.verifiedEkCertChain = chain
	t.verifiedDeviceAttributes = attrs

	if err := t.init(); err != nil {
		if tpm2.IsResourceUnavailableError(err, tpm2.AnyHandle) {
			return nil, ErrTPMProvisioning
		}
		var verifyErr verificationError
		if xerrors.As(err, &verifyErr) {
			return nil, TPMVerificationError{err.Error()}
		}
		return nil, xerrors.Errorf("cannot initialize TPM connection: %w", err)
	}

	succeeded = true
	return t, nil
}

// ConnectToTPM will attempt to connect to a TPM using the currently
// defined connection function. This is used internally by the tpm2
// package when a connection is required, and defaults to
// ConnectToDefaultTPM. This can be overridden with a custom connection
// function.
var ConnectToTPM func() (*Connection, error) = ConnectToDefaultTPM
