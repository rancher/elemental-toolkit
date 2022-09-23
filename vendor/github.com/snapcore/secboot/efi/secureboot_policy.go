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

package efi

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/canonical/go-efilib"
	"github.com/canonical/go-tpm2"
	"github.com/canonical/tcglog-parser"
	"github.com/snapcore/snapd/osutil"

	"golang.org/x/xerrors"

	"go.mozilla.org/pkcs7"

	"github.com/snapcore/secboot/internal/pe1.14"
	secboot_tpm2 "github.com/snapcore/secboot/tpm2"
)

const (
	pkName      = "PK"         // Unicode variable name for the EFI platform key
	kekName     = "KEK"        // Unicode variable name for the EFI KEK database
	dbName      = "db"         // Unicode variable name for the EFI authorized signature database
	dbxName     = "dbx"        // Unicode variable name for the EFI forbidden signature database
	sbStateName = "SecureBoot" // Unicode variable name for the EFI secure boot configuration (enabled/disabled)

	mokListName    = "MokList"    // Unicode variable name for the shim MOK database
	mokSbStateName = "MokSBState" // Unicode variable name for the shim secure boot configuration (validation enabled/disabled)
	sbatName       = "SbatLevel"  // Unicode variable name for the SBAT variable
	shimName       = "Shim"       // Unicode variable name used for recording events when shim's vendor certificate is used for verification

	secureBootPCR = 7 // Secure Boot Policy Measurements PCR

	sbKeySyncExe = "sbkeysync"
)

var (
	shimGuid = efi.MakeGUID(0x605dab50, 0xe046, 0x4300, 0xabb6, [...]uint8{0x3d, 0xd8, 0x10, 0xdd, 0x8b, 0x23}) // SHIM_LOCK_GUID

	oidSha256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}

	efiVarsPath = "/sys/firmware/efi/efivars" // Default mount point for efivarfs
)

type shimFlags int

const (
	// shimHasSbatVerification indicates that the shim
	// binary performs SBAT verification of subsequent loaders,
	// and performs an additional measurement of the SBAT
	// variable.
	shimHasSbatVerification shimFlags = 1 << iota

	// shimVariableAuthorityEventsMatchSpec indicates that shim
	// performs EV_EFI_VARIABLE_AUTHORITY events according to the
	// TCG specification when an image is authenticated with a
	// EFI_SIGNATURE_DATA structure, ie, it has this commit:
	// https://github.com/rhboot/shim/commit/e3325f8100f5a14e0684ff80290e53975de1a5d9
	shimVariableAuthorityEventsMatchSpec
)

type shimImageHandle struct {
	pefile *pe.File
}

func newShimImageHandle(r io.ReaderAt) (*shimImageHandle, error) {
	pefile, err := pe.NewFile(r)
	if err != nil {
		return nil, xerrors.Errorf("cannot decode PE binary: %w", err)
	}

	return &shimImageHandle{pefile}, nil
}

func (s *shimImageHandle) openSection(name string) *pe.Section {
	return s.pefile.Section(name)
}

// readVendorCert obtains the DER encoded built-in vendor certificate from this shim image.
func (s *shimImageHandle) readVendorCert() ([]byte, error) {
	// Shim's vendor certificate is in the .vendor_cert section.
	section := s.openSection(".vendor_cert")
	if section == nil {
		return nil, errors.New("missing .vendor_cert section")
	}

	// Shim's .vendor_cert section starts with a cert_table struct (see shim.c in the shim source)
	sr := io.NewSectionReader(section, 0, 16)

	// Read vendor_cert_size field
	var certSize uint32
	if err := binary.Read(sr, binary.LittleEndian, &certSize); err != nil {
		return nil, xerrors.Errorf("cannot read vendor cert size: %w", err)
	}

	// A size of zero is valid
	if certSize == 0 {
		return nil, nil
	}

	// Skip vendor_dbx_size
	sr.Seek(4, io.SeekCurrent)

	// Read vendor_cert_offset
	var certOffset uint32
	if err := binary.Read(sr, binary.LittleEndian, &certOffset); err != nil {
		return nil, xerrors.Errorf("cannot read vendor cert offset: %w", err)
	}

	sr = io.NewSectionReader(section, int64(certOffset), int64(certSize))
	certData, err := ioutil.ReadAll(sr)
	if err != nil {
		return nil, xerrors.Errorf("cannot read vendor cert data: %w", err)
	}

	return certData, nil
}

func (s *shimImageHandle) hasSbatSection() bool {
	return s.openSection(".sbat") != nil
}

type sigDbUpdateQuirkMode int

const (
	sigDbUpdateQuirkModeNone = iota

	// sigDbUpdateQuirkModeDedupIgnoresOwner enables a mode to compute signature database updates where the firmware
	// doesn't consider 2 EFI_SIGNATURE_DATA entries where only the SignatureOwner fields differ to be unique.
	sigDbUpdateQuirkModeDedupIgnoresOwner
)

// computeDbUpdate appends the EFI signature database update supplied via update to the signature database supplied via orig, filtering
// out EFI_SIGNATURE_DATA entries that are already in orig and then returning the result.
func computeDbUpdate(base, update io.Reader, quirkMode sigDbUpdateQuirkMode) ([]byte, error) {
	// Skip over authentication header
	_, err := efi.ReadTimeBasedVariableAuthentication(update)
	if err != nil {
		return nil, xerrors.Errorf("cannot decode EFI_VARIABLE_AUTHENTICATION_2 structure: %w", err)
	}

	baseDb, err := efi.ReadSignatureDatabase(base)
	if err != nil {
		return nil, xerrors.Errorf("cannot decode base signature database: %w", err)
	}

	updateDb, err := efi.ReadSignatureDatabase(update)
	if err != nil {
		return nil, xerrors.Errorf("cannot decode signature database update: %w", err)
	}

	var filtered efi.SignatureDatabase

	for _, ul := range updateDb {
		var newSigs []*efi.SignatureData

		for _, us := range ul.Signatures {
			isNewSig := true

		BaseLoop:
			for _, l := range baseDb {
				if l.Type != ul.Type {
					// Different signature type
					continue
				}

				for _, s := range l.Signatures {
					switch quirkMode {
					case sigDbUpdateQuirkModeNone:
						if us.Equal(s) {
							isNewSig = false
						}
					case sigDbUpdateQuirkModeDedupIgnoresOwner:
						if bytes.Equal(us.Data, s.Data) {
							isNewSig = false
						}
					}
					if !isNewSig {
						break BaseLoop
					}
				}
			}

			if isNewSig {
				newSigs = append(newSigs, us)
			}
		}

		if len(newSigs) > 0 {
			filtered = append(filtered, &efi.SignatureList{Type: ul.Type, Header: ul.Header, Signatures: newSigs})
		}
	}

	baseDb = append(baseDb, filtered...)

	var buf bytes.Buffer
	if err := baseDb.Write(&buf); err != nil {
		return nil, xerrors.Errorf("cannot encode update signature database: %w", err)
	}

	return buf.Bytes(), nil
}

// secureBootDbUpdate corresponds to an on-disk EFI signature database update.
type secureBootDbUpdate struct {
	db   string
	path string
}

// buildSignatureDbUpdateList builds a list of EFI signature database updates that will be applied by sbkeysync when executed with
// the provided key stores.
func buildSignatureDbUpdateList(keystores []string) ([]*secureBootDbUpdate, error) {
	if len(keystores) == 0 {
		// Nothing to do
		return nil, nil
	}

	// Run sbkeysync in dry run mode to build a list of updates it will try to append. It will only try to append an update that
	// contains keys which don't currently exist in the firmware database.
	// FIXME: This isn't a guarantee that the update is actually applicable because it could fail a signature check. We should
	// probably filter updates out if they obviously won't apply.
	var updates []*secureBootDbUpdate

	sbKeySync, err := exec.LookPath(sbKeySyncExe)
	if err != nil {
		return nil, xerrors.Errorf("lookup failed %s: %w", sbKeySyncExe, err)
	}

	args := []string{"--dry-run", "--verbose", "--no-default-keystores", "--efivars-path", efiVarsPath}
	for _, ks := range keystores {
		args = append(args, "--keystore", ks)
	}

	out, err := osutil.StreamCommand(sbKeySync, args...)
	if err != nil {
		return nil, xerrors.Errorf("cannot execute command: %v", err)
	}

	scanner := bufio.NewScanner(out)
	seenNewKeysHeader := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "New keys in filesystem:" {
			seenNewKeysHeader = true
			continue
		}
		if !seenNewKeysHeader {
			continue
		}
		line = strings.TrimSpace(line)
		for _, ks := range keystores {
			rel, err := filepath.Rel(ks, line)
			if err != nil {
				continue
			}
			if strings.HasPrefix("..", rel) {
				continue
			}
			updates = append(updates, &secureBootDbUpdate{db: filepath.Dir(rel), path: line})
		}
	}

	return updates, nil
}

func isSecureBootEvent(event *tcglog.Event) bool {
	return event.PCRIndex == secureBootPCR
}

// isSecureBootConfigMeasurementEvent determines if event corresponds to the measurement of secure
// boot configuration.
func isSecureBootConfigMeasurementEvent(event *tcglog.Event) bool {
	return isSecureBootEvent(event) && event.EventType == tcglog.EventTypeEFIVariableDriverConfig
}

// isDbMeasurementEvent determines if event corresponds to the measurement of the UEFI authorized
// signature database.
func isDbMeasurementEvent(event *tcglog.Event) bool {
	if !isSecureBootConfigMeasurementEvent(event) {
		return false
	}

	data := event.Data.(*tcglog.EFIVariableData)
	return data.VariableName == efi.ImageSecurityDatabaseGuid && data.UnicodeName == dbName
}

// isSignatureDatabaseMeasurementEVent determines if event corresponds to the measurement of one
// of the UEFI signature databases.
func isSignatureDatabaseMeasurementEvent(event *tcglog.Event) bool {
	if !isSecureBootConfigMeasurementEvent(event) {
		return false
	}

	data := event.Data.(*tcglog.EFIVariableData)
	switch {
	case data.VariableName == efi.GlobalVariable && data.UnicodeName == pkName:
		return true
	case data.VariableName == efi.GlobalVariable && data.UnicodeName == kekName:
		return true
	case data.VariableName == efi.ImageSecurityDatabaseGuid:
		return true
	default:
		return false
	}
}

// isVerificationEvent determines if event corresponds to the verification of a EFI image.
func isVerificationEvent(event *tcglog.Event) bool {
	return isSecureBootEvent(event) && event.EventType == tcglog.EventTypeEFIVariableAuthority
}

// isShimExecutable determines if the EFI executable read from r looks like a valid shim binary (ie, it has a ".vendor_cert" section.
func isShimExecutable(r io.ReaderAt) (bool, error) {
	pefile, err := pe.NewFile(r)
	if err != nil {
		return false, xerrors.Errorf("cannot decode PE binary: %w", err)
	}
	return pefile.Section(".vendor_cert") != nil, nil
}

// SecureBootPolicyProfileParams provide the arguments to AddSecureBootPolicyProfile.
type SecureBootPolicyProfileParams struct {
	// PCRAlgorithm is the algorithm for which to compute PCR digests for. TPMs compliant with the "TCG PC Client Platform TPM Profile
	// (PTP) Specification" Level 00, Revision 01.03 v22, May 22 2017 are required to support tpm2.HashAlgorithmSHA1 and
	// tpm2.HashAlgorithmSHA256. Support for other digest algorithms is optional.
	PCRAlgorithm tpm2.HashAlgorithmId

	// LoadSequences is a list of EFI image load sequences for which to compute PCR digests for.
	LoadSequences []*ImageLoadEvent

	// SignatureDbUpdateKeystores is a list of directories containing EFI signature database updates for which to compute PCR digests
	// for. These directories are passed to sbkeysync using the --keystore option.
	SignatureDbUpdateKeystores []string

	// Environment is an optional parameter that allows the caller to provide
	// a custom EFI environment. If not set, the host's normal environment will
	// be used
	Environment HostEnvironment
}

// secureBootDb corresponds to a EFI signature database.
type secureBootDb struct {
	variableName efi.GUID
	unicodeName  string
	db           efi.SignatureDatabase
}

// secureBootDbSet corresponds to a set of EFI signature databases.
type secureBootDbSet struct {
	uefiDb *secureBootDb
	mokDb  *secureBootDb
	shimDb *secureBootDb
}

type secureBootAuthority struct {
	signature *efi.SignatureData
	source    *secureBootDb
}

type authenticodeSignerAndIntermediates struct {
	signer        *x509.Certificate
	intermediates *x509.CertPool
}

// secureBootPolicyGen is the main structure involved with computing secure boot policy PCR digests. It is essentially just
// a container for SecureBootPolicyProfileParams - per-branch context is maintained in secureBootPolicyGenBranch instead.
type secureBootPolicyGen struct {
	pcrAlgorithm  tpm2.HashAlgorithmId
	env           HostEnvironment
	loadSequences []*ImageLoadEvent

	events       []*tcglog.Event
	sigDbUpdates []*secureBootDbUpdate
}

// secureBootPolicyGenBranch represents a branch of a PCRProtectionProfile. It contains its own PCRProtectionProfile in to which
// instructions can be recorded, as well as some other context associated with this branch.
type secureBootPolicyGenBranch struct {
	gen *secureBootPolicyGen

	profile     *secboot_tpm2.PCRProtectionProfile // The PCR profile containing the instructions for this branch
	subBranches []*secureBootPolicyGenBranch       // Sub-branches, if this has been branched

	dbUpdateLevel              int             // The number of EFI signature database updates applied in this branch
	dbSet                      secureBootDbSet // The signature database set associated with this branch
	firmwareVerificationEvents tpm2.DigestList // The verification events recorded by firmware in this branch
	shimVerificationEvents     tpm2.DigestList // The verification events recorded by shim in this branch
	shimFlags                  shimFlags       // Flags associated with shim in this branch
}

// branch creates a branch point in the current branch if one doesn't exist already (although inserting this branch point with
// PCRProtectionProfile.AddProfileOR is deferred until later), and creates a new sub-branch at the current branch point. Once
// this has been called, no more instructions can be inserted in to the current branch.
func (b *secureBootPolicyGenBranch) branch() *secureBootPolicyGenBranch {
	c := &secureBootPolicyGenBranch{gen: b.gen, profile: secboot_tpm2.NewPCRProtectionProfile()}
	b.subBranches = append(b.subBranches, c)

	// Preserve the context associated with this branch
	c.dbUpdateLevel = b.dbUpdateLevel
	c.dbSet = b.dbSet
	c.firmwareVerificationEvents = make(tpm2.DigestList, len(b.firmwareVerificationEvents))
	copy(c.firmwareVerificationEvents, b.firmwareVerificationEvents)
	c.shimVerificationEvents = make(tpm2.DigestList, len(b.shimVerificationEvents))
	copy(c.shimVerificationEvents, b.shimVerificationEvents)
	c.shimFlags = b.shimFlags

	return c
}

// extendMeasurement extends the supplied digest to this branch.
func (b *secureBootPolicyGenBranch) extendMeasurement(digest tpm2.Digest) {
	if len(b.subBranches) > 0 {
		panic("This branch has already been branched")
	}
	b.profile.ExtendPCR(b.gen.pcrAlgorithm, secureBootPCR, digest)
}

// extendVerificationMeasurement extends the supplied digest and records that the digest has been measured by the specified source in
// to this branch.
func (b *secureBootPolicyGenBranch) extendVerificationMeasurement(digest tpm2.Digest, source ImageLoadEventSource) {
	var digests *tpm2.DigestList
	switch source {
	case Firmware:
		digests = &b.firmwareVerificationEvents
	case Shim:
		digests = &b.shimVerificationEvents
	}
	*digests = append(*digests, digest)
	b.extendMeasurement(digest)
}

// extendFirmwareVerificationMeasurement extends the supplied digest and records that the digest has been measured by the firmware
// in to this branch.
func (b *secureBootPolicyGenBranch) extendFirmwareVerificationMeasurement(digest tpm2.Digest) {
	b.extendVerificationMeasurement(digest, Firmware)
}

// omputeAndExtendVariableMeasurement computes a EFI variable measurement from the supplied arguments and extends that to
// this branch.
func (b *secureBootPolicyGenBranch) computeAndExtendVariableMeasurement(varName efi.GUID, unicodeName string, varData []byte) {
	b.extendMeasurement(tcglog.ComputeEFIVariableDataDigest(b.gen.pcrAlgorithm.GetHash(), unicodeName, varName, varData))
}

// processSignatureDbMeasurementEvent computes a EFI signature database measurement for the specified database and with the supplied
// updates, and then extends that in to this branch.
func (b *secureBootPolicyGenBranch) processSignatureDbMeasurementEvent(guid efi.GUID, name string, updates []*secureBootDbUpdate, updateQuirkMode sigDbUpdateQuirkMode) ([]byte, error) {
	db, _, err := b.gen.env.ReadVar(name, guid)
	if err != nil && err != efi.ErrVarNotExist {
		return nil, xerrors.Errorf("cannot read current variable: %w", err)
	}

	for _, u := range updates {
		if u.db != name {
			continue
		}
		if f, err := os.Open(u.path); err != nil {
			return nil, xerrors.Errorf("cannot open signature DB update: %w", err)
		} else if d, err := computeDbUpdate(bytes.NewReader(db), f, updateQuirkMode); err != nil {
			return nil, xerrors.Errorf("cannot compute signature DB update for %s: %w", u.path, err)
		} else {
			db = d
		}
	}

	b.computeAndExtendVariableMeasurement(guid, name, db)
	return db, nil
}

// processDbMeasurementEvent computes a measurement of the EFI authorized signature database with the supplied updates applied and
// then extends that in to this branch. The branch context is then updated to contain a list of signatures associated with the
// resulting authorized signature database contents, which is used later on when computing verification events in
// secureBootPolicyGen.computeAndExtendVerificationMeasurement.
func (b *secureBootPolicyGenBranch) processDbMeasurementEvent(updates []*secureBootDbUpdate, updateQuirkMode sigDbUpdateQuirkMode) error {
	db, err := b.processSignatureDbMeasurementEvent(efi.ImageSecurityDatabaseGuid, dbName, updates, updateQuirkMode)
	if err != nil {
		return err
	}

	sigDb, err := efi.ReadSignatureDatabase(bytes.NewReader(db))
	if err != nil {
		return xerrors.Errorf("cannot decode DB contents: %w", err)
	}

	b.dbSet.uefiDb = &secureBootDb{variableName: efi.ImageSecurityDatabaseGuid, unicodeName: dbName, db: sigDb}

	return nil
}

// processPreOSEvents iterates over the pre-OS secure boot policy events contained within the supplied list of events and extends
// these in to this branch. For events corresponding to the measurement of EFI signature databases, measurements are computed based
// on the current contents of each database with the supplied updates applied.
//
// Processing of the list of events stops after transitioning from pre-OS to OS-present. This transition is indicated when an
// EV_SEPARATOR event has been measured to any of PCRs 0-6 AND PCR 7. This handles 2 different firmware behaviours:
// - Some firmware implementations signal the transition by measuring EV_SEPARATOR events to PCRs 0-7 at the same time.
// - Other firmware implementations measure a EV_SEPARATOR event to PCR 7 immediately after measuring the secure boot
//   configuration, which is before the transition to OS-present. In this case, processing of pre-OS events in PCR 7
//   must continue until an EV_SEPARATOR event is encountered in PCRs 0-6. On firmware implmentations that support
//   secure boot verification of EFI drivers, these verification events will be recorded to PCR 7 after the
//   EV_SEPARATOR event in PCR 7 but before the EV_SEPARATOR events in PCRs 0-6.
func (b *secureBootPolicyGenBranch) processPreOSEvents(events []*tcglog.Event, sigDbUpdates []*secureBootDbUpdate, sigDbUpdateQuirkMode sigDbUpdateQuirkMode) error {
	osPresent := false
	seenSecureBootPCRSeparator := false

	for len(events) > 0 {
		e := events[0]
		events = events[1:]
		switch {
		case e.PCRIndex < secureBootPCR && e.EventType == tcglog.EventTypeSeparator:
			osPresent = true
		case isDbMeasurementEvent(e):
			// This is the db variable - requires special handling because it updates context
			// for this branch.
			if err := b.processDbMeasurementEvent(sigDbUpdates, sigDbUpdateQuirkMode); err != nil {
				return xerrors.Errorf("cannot process db measurement event: %w", err)
			}
		case isSignatureDatabaseMeasurementEvent(e):
			// This is any signature database variable other than db.
			data := e.Data.(*tcglog.EFIVariableData)
			if _, err := b.processSignatureDbMeasurementEvent(data.VariableName, data.UnicodeName, sigDbUpdates, sigDbUpdateQuirkMode); err != nil {
				return xerrors.Errorf("cannot process %s measurement event: %w", data.UnicodeName, err)
			}
		case isVerificationEvent(e):
			// This is a verification event corresponding to a UEFI driver or system
			// preparation application.
			b.extendFirmwareVerificationMeasurement(tpm2.Digest(e.Digests[b.gen.pcrAlgorithm]))
		case isSecureBootEvent(e):
			// This is any secure boot event that isn't a verification event or signature
			// database measurement. Secure boot configuration variables that aren't signature
			// databases are volatile variables mirrored by boot services code from a
			// non-volatile boot services only variable (eg, SecureBoot or DeployedMode).
			// The non-volatile variable can only be accessed by boot services code, so
			// always replay the log digest.
			b.extendMeasurement(tpm2.Digest(e.Digests[b.gen.pcrAlgorithm]))
			if e.EventType == tcglog.EventTypeSeparator {
				seenSecureBootPCRSeparator = true
			}
		}

		if osPresent && seenSecureBootPCRSeparator {
			break
		}
	}

	return nil
}

// processShimExecutableLaunch updates the context in this branch with the supplied shim vendor certificate so that it can be used
// later on when computing verification events in secureBootPolicyGenBranch.computeAndExtendVerificationMeasurement.
func (b *secureBootPolicyGenBranch) processShimExecutableLaunch(vendorCert []byte, flags shimFlags) {
	if b.profile == nil {
		// This branch is going to be excluded because it is unbootable.
		return
	}

	if flags&shimHasSbatVerification > 0 {
		// XXX: This is a bit of a hack, just so that we are compatible with
		// the latest shim. Some things to note:
		// - SBAT-capable shim will initialize the SBAT variable to a known
		//   (compiled in) payload if the variable doesn't exist, has an older
		//   payload or doesn't have the correct attributes. "Older" right now
		//   is determined by checking a timestamp in the payload whilst the
		//   variable is BS+NV and not-authenticated, but would be determined by
		//   the signature timestamp for an authenticated variable in the future.
		// - The variable is initialized as BS+NV and then mirrored to a RT
		//   variable by a SBAT-capable shim, which means we don't know what
		//   the current variable value is if we are pre-computing a PCR policy
		//   on a system that was booted with a pre-SBAT shim. This isn't a
		//   problem right now because there is only one payload, but will be
		//   a problem in the future as updates are published if the variable
		//   remains a non-authenticated BS+NV variable as opposed to a RT+BS+NV
		//   authenticated variable.
		// - In the future and in order to do this properly, we need an authoritative
		//   source for the current variable value (eg, event log for BS+NV variable
		//   or current variable value for RT+BS+NV, like we do for other
		//   configuration).
		// - Shim doesn't provide a way to audit the compiled-in SBAT payload, so
		//   we don't have a way to introspect what it will set it to, although
		//   we know what it is right now.
		// - Future PCR value computation will be a bit more complicated than it
		//   is now - imagine if you have 2 shim's with different built-in SBAT
		//   payloads, and those are both different to the current SBAT value.
		//   Because shim will overwrite the SBAT variable if its built-in
		//   payload is newer, booting with one shim may affect the PCR values
		//   associated with a branch that has a different shim.
		b.computeAndExtendVariableMeasurement(shimGuid, sbatName, []byte("sbat,1,2021030218\n"))
	}

	b.dbSet.shimDb = &secureBootDb{variableName: shimGuid, unicodeName: shimName}
	if vendorCert != nil {
		b.dbSet.shimDb.db = efi.SignatureDatabase{&efi.SignatureList{Type: efi.CertX509Guid, Signatures: []*efi.SignatureData{{Data: vendorCert}}}}
	}
	b.shimVerificationEvents = nil
	b.shimFlags = flags
}

// hasVerificationEventBeenMeasuredBy determines whether the verification event with the associated digest has been measured by the
// supplied source already in this branch.
func (b *secureBootPolicyGenBranch) hasVerificationEventBeenMeasuredBy(digest tpm2.Digest, source ImageLoadEventSource) bool {
	var digests *tpm2.DigestList
	switch source {
	case Firmware:
		digests = &b.firmwareVerificationEvents
	case Shim:
		digests = &b.shimVerificationEvents
	}
	for _, d := range *digests {
		if bytes.Equal(d, digest) {
			return true
		}
	}
	return false
}

// computeAndExtendVerificationMeasurement computes a measurement for the the authentication of an EFI image using the supplied
// signatures and extends that in to this branch. If the computed measurement has already been measured by the specified source, then
// it will not be measured again.
//
// In order to compute the measurement, the CA certificate that will be used to authenticate the image using the supplied signatures,
// and the source of that certificate, needs to be determined. If the image is not signed with an authority that is trusted by a CA
// certificate that exists in this branch, then this branch will be marked as unbootable and it will be omitted from the final PCR
// profile.
func (b *secureBootPolicyGenBranch) computeAndExtendVerificationMeasurement(sigs []*authenticodeSignerAndIntermediates, source ImageLoadEventSource) error {
	if b.profile == nil {
		// This branch is going to be excluded because it is unbootable.
		return nil
	}

	dbs := []*secureBootDb{b.dbSet.uefiDb}
	if source == Shim {
		if b.dbSet.shimDb == nil {
			return errors.New("shim specified as event source without a shim executable appearing in preceding events")
		}
		dbs = append(dbs, b.dbSet.mokDb, b.dbSet.shimDb)
	}

	var authority *secureBootAuthority

	// To determine what CA certificate will be used to authenticate this image, iterate over the signatures in the order in which they
	// appear in the binary in this outer loop. Iterating over the CA certificates occurs in an inner loop. This behaviour isn't defined
	// in the UEFI specification but it matches EDK2 and the firmware on the Intel NUC. If an implementation iterates over the CA
	// certificates in an outer loop and the signatures in an inner loop, then this may produce the wrong result.
Outer:
	for _, sig := range sigs {
		for _, db := range dbs {
			if db == nil {
				continue
			}

			// Iterate over ESLs
			for _, l := range db.db {
				// Ignore ESLs that aren't X509 certificates
				if l.Type != efi.CertX509Guid {
					continue
				}

				// Shouldn't happen, but just in case...
				if len(l.Signatures) == 0 {
					continue
				}

				ca, err := x509.ParseCertificate(l.Signatures[0].Data)
				if err != nil {
					continue
				}

				// XXX: This only works if the CA certificate is also the code signing
				// certificate, or it directly signs the code signing certificate. Ideally
				// we would use x509.Certificate.Verify here, but there is no way to turn
				// off time checking and UEFI doesn't consider expired certificates invalid.
				if bytes.Equal(ca.Raw, sig.signer.Raw) {
					// The signer certificate is the CA
					authority = &secureBootAuthority{signature: l.Signatures[0], source: db}
					break Outer
				}
				if err := sig.signer.CheckSignatureFrom(ca); err == nil {
					// The signer certificate is signed by the CA
					authority = &secureBootAuthority{signature: l.Signatures[0], source: db}
					break Outer
				}
			}
		}
	}

	if authority == nil {
		// Mark this branch as unbootable by clearing its PCR profile
		b.profile = nil
		return nil
	}

	// Serialize authority certificate for measurement
	var varData *bytes.Buffer
	switch {
	case source == Shim && (b.shimFlags&shimVariableAuthorityEventsMatchSpec == 0 || authority.source == b.dbSet.shimDb):
		// Shim measures the certificate data rather than the entire EFI_SIGNATURE_DATA
		// in some circumstances.
		varData = bytes.NewBuffer(authority.signature.Data)
	default:
		// Firmware always measures the entire EFI_SIGNATURE_DATA including the SignatureOwner,
		// and newer versions of shim do in some circumstances.
		varData = new(bytes.Buffer)
		if err := authority.signature.Write(varData); err != nil {
			return xerrors.Errorf("cannot encode EFI_SIGNATURE_DATA for authority: %w", err)
		}
	}

	// Create event data, compute digest and perform extension for verification of this executable
	digest := tcglog.ComputeEFIVariableDataDigest(
		b.gen.pcrAlgorithm.GetHash(),
		authority.source.unicodeName,
		authority.source.variableName,
		varData.Bytes())

	// Don't measure events that have already been measured
	if b.hasVerificationEventBeenMeasuredBy(digest, source) {
		return nil
	}
	b.extendVerificationMeasurement(digest, source)
	return nil
}

// sbLoadEventAndBranches binds together a ImageLoadEvent and the branches that the event needs to be applied to.
type sbLoadEventAndBranches struct {
	event    *ImageLoadEvent
	branches []*secureBootPolicyGenBranch
}

func (e *sbLoadEventAndBranches) branch(event *ImageLoadEvent) *sbLoadEventAndBranches {
	var branches []*secureBootPolicyGenBranch
	for _, b := range e.branches {
		if b.profile == nil {
			continue
		}
		branches = append(branches, b.branch())
	}
	return &sbLoadEventAndBranches{event, branches}
}

// computeAndExtendVerificationMeasurement computes a measurement for the the authentication of the EFI image obtained from r and
// extends that to the supplied branches. If the computed measurement has already been measured by the specified source in a branch,
// then it will not be measured again.
//
// In order to compute the measurement for each branch, the CA certificate that will be used to authenticate the image and the
// source of that certificate needs to be determined. If the image is not signed with an authority that is trusted by a CA
// certificate for a particular branch, then that branch will be marked as unbootable and it will be omitted from the final PCR
// profile.
func (g *secureBootPolicyGen) computeAndExtendVerificationMeasurement(branches []*secureBootPolicyGenBranch, r io.ReaderAt, source ImageLoadEventSource) error {
	pefile, err := pe.NewFile(r)
	if err != nil {
		return xerrors.Errorf("cannot decode PE binary: %w", err)
	}

	// Obtain security directory entry from optional header
	var dd []pe.DataDirectory
	switch oh := pefile.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		dd = oh.DataDirectory[0:oh.NumberOfRvaAndSizes]
	case *pe.OptionalHeader64:
		dd = oh.DataDirectory[0:oh.NumberOfRvaAndSizes]
	default:
		return errors.New("cannot obtain security directory entry from PE binary: no optional header")
	}

	if len(dd) <= certTableIndex {
		return errors.New("cannot obtain security directory entry from PE binary: invalid number of data directories")
	}

	// Create a reader for the security directory entry, which points to a WIN_CERTIFICATE struct
	certReader := io.NewSectionReader(r, int64(dd[certTableIndex].VirtualAddress), int64(dd[certTableIndex].Size))

	// Binaries can have multiple signers - this is achieved using multiple single-signed Authenticode signatures - see section 32.5.3.3
	// ("Secure Boot and Driver Signing - UEFI Image Validation - Signature Database Update - Authorization Process") of the UEFI
	// Specification, version 2.8.
	var sigs []*authenticodeSignerAndIntermediates

Outer:
	for {
		// Signatures in this section are 8-byte aligned - see the PE spec:
		// https://docs.microsoft.com/en-us/windows/win32/debug/pe-format#the-attribute-certificate-table-image-only
		off, _ := certReader.Seek(0, io.SeekCurrent)
		alignSize := (8 - (off & 7)) % 8
		certReader.Seek(alignSize, io.SeekCurrent)

		c, err := efi.ReadWinCertificate(certReader)
		switch {
		case xerrors.Is(err, io.EOF):
			break Outer
		case err != nil:
			return xerrors.Errorf("cannot decode WIN_CERTIFICATE from security directory entry of PE binary: %w", err)
		}

		if _, ok := c.(efi.WinCertificateAuthenticode); !ok {
			return errors.New("unexpected WIN_CERTIFICATE type: not an Authenticode signature")
		}

		// Decode the signature
		p7, err := pkcs7.Parse(c.(efi.WinCertificateAuthenticode))
		if err != nil {
			return xerrors.Errorf("cannot decode signature: %w", err)
		}

		// Grab the certificate of the signer
		signer := p7.GetOnlySigner()
		if signer == nil {
			return errors.New("cannot obtain signer certificate from signature")
		}

		// Reject any signature with a digest algorithm other than SHA256, as that's the only algorithm used for binaries we're
		// expected to support, and therefore required by the UEFI implementation.
		if !p7.Signers[0].DigestAlgorithm.Algorithm.Equal(oidSha256) {
			return errors.New("signature has unexpected digest algorithm")
		}

		// Grab all of the certificates in the signature and populate an intermediates pool
		intermediates := x509.NewCertPool()
		for _, c := range p7.Certificates {
			intermediates.AddCert(c)
		}

		sigs = append(sigs, &authenticodeSignerAndIntermediates{signer: p7.GetOnlySigner(), intermediates: intermediates})
	}

	if len(sigs) == 0 {
		return errors.New("no Authenticode signatures")
	}

	for _, b := range branches {
		if err := b.computeAndExtendVerificationMeasurement(sigs, source); err != nil {
			return err
		}
	}

	return nil
}

// processShimExecutableLaunch extracts the vendor certificate from the shim executable read from r, and then updates the specified
// branches to contain a reference to the vendor certificate so that it can be used later on when computing verification events in
// secureBootPolicyGen.computeAndExtendVerificationMeasurement for images that are authenticated by shim.
func (g *secureBootPolicyGen) processShimExecutableLaunch(branches []*secureBootPolicyGenBranch, shim *shimImageHandle) error {
	// Extract this shim's vendor cert
	vendorCert, err := shim.readVendorCert()
	if err != nil {
		return xerrors.Errorf("cannot extract vendor certificate: %w", err)
	}

	var flags shimFlags

	// Check if this shim has a .sbat section. We use this to make some assumptions
	// about shim's behaviour below.
	hasSbatSection := shim.hasSbatSection()

	if hasSbatSection {
		// If this shim has a .sbat section, assume it also does SBAT verification.
		// This isn't a perfect heuristic, but nobody is adding a .sbat section to a
		// pre-SBAT version of shim and then signing it, so it doesn't matter.
		flags |= shimHasSbatVerification

		// There isn't a good heuristic for this, but at least none of Canonical's
		// pre-SBAT shim's had the fix for this, and all SBAT capable shims do
		// have this fix.
		// XXX: It's possible that this is broken for shims that weren't signed
		//  for Canonical.
		flags |= shimVariableAuthorityEventsMatchSpec
	}

	for _, b := range branches {
		b.processShimExecutableLaunch(vendorCert, flags)
	}

	return nil
}

// processOSLoadEvent computes a measurement associated with the supplied image load event and extends this to the specified branches.
// If the image load corresponds to shim, then some additional processing is performed to extract the included vendor certificate
// (see secureBootPolicyGen.processShimExecutableLaunch).
func (g *secureBootPolicyGen) processOSLoadEvent(branches []*secureBootPolicyGenBranch, event *ImageLoadEvent) error {
	r, err := event.Image.Open()
	if err != nil {
		return xerrors.Errorf("cannot open image: %w", err)
	}
	defer r.Close()

	isShim, err := isShimExecutable(r)
	if err != nil {
		return xerrors.Errorf("cannot determine image type: %w", err)
	}

	if err := g.computeAndExtendVerificationMeasurement(branches, r, event.Source); err != nil {
		return xerrors.Errorf("cannot compute load verification event: %w", err)
	}

	if !isShim {
		return nil
	}

	shim, err := newShimImageHandle(r)
	if err != nil {
		return xerrors.Errorf("cannot create handle for shim image: %w", err)
	}

	if err := g.processShimExecutableLaunch(branches, shim); err != nil {
		return xerrors.Errorf("cannot process shim executable: %w", err)
	}

	return nil
}

// run takes a TCG event log and builds a PCR profile from the supplied configuration (see SecureBootPolicyProfileParams)
func (g *secureBootPolicyGen) run(profile *secboot_tpm2.PCRProtectionProfile, sigDbUpdateQuirkMode sigDbUpdateQuirkMode) error {
	// Process the pre-OS events for the current signature DB and then with each pending update applied
	// in turn.
	var roots []*secureBootPolicyGenBranch
	for i := 0; i <= len(g.sigDbUpdates); i++ {
		branch := &secureBootPolicyGenBranch{gen: g, profile: secboot_tpm2.NewPCRProtectionProfile(), dbUpdateLevel: i}
		if err := branch.processPreOSEvents(g.events, g.sigDbUpdates[0:i], sigDbUpdateQuirkMode); err != nil {
			return xerrors.Errorf("cannot process pre-OS events from event log: %w", err)
		}
		roots = append(roots, branch)
	}

	allBranches := make([]*secureBootPolicyGenBranch, len(roots))
	copy(allBranches, roots)

	var loadEvents []*sbLoadEventAndBranches
	var nextLoadEvents []*sbLoadEventAndBranches

	if len(g.loadSequences) == 1 {
		loadEvents = append(loadEvents, &sbLoadEventAndBranches{event: g.loadSequences[0], branches: roots})
	} else {
		for _, e := range g.loadSequences {
			var branches []*secureBootPolicyGenBranch
			for _, b := range roots {
				branches = append(branches, b.branch())
			}
			allBranches = append(allBranches, branches...)
			loadEvents = append(loadEvents, &sbLoadEventAndBranches{event: e, branches: branches})
		}
	}

	for len(loadEvents) > 0 {
		e := loadEvents[0]
		loadEvents = loadEvents[1:]

		if err := g.processOSLoadEvent(e.branches, e.event); err != nil {
			return xerrors.Errorf("cannot process OS load event for %s: %w", e.event.Image, err)
		}

		if len(e.event.Next) == 1 {
			nextLoadEvents = append(nextLoadEvents, &sbLoadEventAndBranches{event: e.event.Next[0], branches: e.branches})
		} else {
			for _, n := range e.event.Next {
				ne := e.branch(n)
				allBranches = append(allBranches, ne.branches...)
				nextLoadEvents = append(nextLoadEvents, ne)
			}
		}

		if len(loadEvents) == 0 {
			loadEvents = nextLoadEvents
			nextLoadEvents = nil
		}
	}

	for i := len(allBranches) - 1; i >= 0; i-- {
		b := allBranches[i]

		if len(b.subBranches) == 0 {
			// This is a leaf branch
			continue
		}

		var subProfiles []*secboot_tpm2.PCRProtectionProfile
		for _, sb := range b.subBranches {
			if sb.profile == nil {
				// This sub-branch has been marked unbootable
				continue
			}
			subProfiles = append(subProfiles, sb.profile)
		}

		if len(subProfiles) == 0 {
			// All sub branches are unbootable, so ensure our parent branch omits us too.
			b.profile = nil
			continue
		}

		b.profile.AddProfileOR(subProfiles...)
	}

	validPathsForCurrentDb := false
	var subProfiles []*secboot_tpm2.PCRProtectionProfile
	for _, b := range roots {
		if b.profile == nil {
			// This branch has no bootable paths
			continue
		}
		if b.dbUpdateLevel == 0 {
			validPathsForCurrentDb = true
		}
		subProfiles = append(subProfiles, b.profile)
	}

	if !validPathsForCurrentDb {
		return errors.New("no bootable paths with current EFI signature database")
	}

	profile.AddProfileOR(subProfiles...)

	return nil
}

// AddSecureBootPolicyProfile adds the UEFI secure boot policy profile to the provided PCR protection profile, in order to generate
// a PCR policy that restricts access to a sealed key to a set of UEFI secure boot policies measured to PCR 7. The secure boot policy
// information that is measured to PCR 7 is defined in section 2.3.4.8 of the "TCG PC Client Platform Firmware Profile Specification".
//
// This function can only be called if the current boot was performed with secure boot enabled. An error will be returned if the
// current boot was performed with secure boot disabled. It can only generate a PCR profile that will work when secure boot is
// enabled.
//
// The secure boot policy measurements include events that correspond to the authentication of loaded EFI images, and those events
// record the certificate of the authorities used to authenticate these images. The params argument allows the generated PCR policy
// to be restricted to a specific set of chains of trust by specifying EFI image load sequences via the LoadSequences field. This
// function will compute the measurements associated with the authentication of these load sequences. Each of the Image instances
// reachable from the LoadSequences field of params must correspond to an EFI image with one or more Authenticode signatures. These
// signatures are used to determine the CA certificate that will be used to authenticate them in order to compute authentication
// meausurement events. The digest algorithm of the Authenticode signatures must be SHA256. If there are no signatures, or the
// binary's certificate table contains non-Authenticode entries, or contains any Authenticode signatures with a digest algorithm other
// than SHA256, then an error will be returned. Note that this function assumes that any signatures are correct and does not ensure
// that they are so - it only determines if there is a chain of trust beween the signing certificate and a CA certificate in order to
// determine which certificate will be used for authentication, and what the source of that certificate is (for UEFI images that are
// loaded by shim).
//
// If none of the sequences in the LoadSequences field of params can be authenticated by the current authorized signature database
// contents, then an error will be returned.
//
// This function does not support computing measurements for images that are authenticated by an image digest rather than an
// Authenticode signature. If an image has a signature where the signer has a chain of trust to a CA certificate in the authorized
// signature database (or shim's vendor certificate) but that image is authenticated because an image digest is present in the
// authorized signature database instead, then this function will generate a PCR profile that is incorrect.
//
// If an image has a signature that can be authenticated by multiple CA certificates in the authorized signature database, this
// function assumes that the firmware will try the CA certificates in the order in which they appear in the database and authenticate
// the image with the first valid certificate. If the firmware does not do this, then this function may generate a PCR profile that is
// incorrect for binaries that have a signature that can be authenticated by more than one CA certificate. Note that the structure of
// the signature database means that it can only really be iterated in one direction anyway.
//
// For images with multiple Authenticode signatures, this function assumes that the device's firmware will iterate over the signatures
// in the order in which they appear in the binary's certificate table in an outer loop during image authentication (ie, for each
// signature, attempt to authenticate the binary using one of the CA certificates). If a device's firmware iterates over the
// authorized signature database in an outer loop instead (ie, for each CA certificate, attempt to authenticate the binary using one
// of its signatures), then this function may generate a PCR profile that is incorrect for binaries that have multiple signatures
// where both signers have a chain of trust to a different CA certificate but the signatures appear in a different order to which
// their CA certificates are enrolled.
//
// This function does not consider the contents of the forbidden signature database. This is most relevant for images with multiple
// signatures. If an image has more than one signature where the signing certificates have chains of trust to different CA
// certificates, but the first signature is not used to authenticate the image because one of the certificates in its chain is
// blacklisted, then this function will generate a PCR profile that is incorrect.
//
// In determining whether a signing certificate has a chain of trust to a CA certificate, this function expects there to be a direct
// relationship between the CA certificate and signing certificate. It does not currently detect that there is a chain of trust if
// intermediate certificates form part of the chain. This is most relevant for images with multiple signatures. If an image has more
// than one signature where the signing certificate have chains of trust to different CA certificate, but the first signature's chain
// involves intermediate certificates, then this function will generate a PCR profile that is incorrect.
//
// This function does not support computing measurements for images that are authenticated by shim using a machine owner key (MOK).
//
// The secure boot policy measurements include the secure boot configuration, which includes the contents of the UEFI signature
// databases. In order to support atomic updates of these databases with the sbkeysync tool, it is possible to generate a PCR policy
// computed from pending signature database updates. This can be done by supplying the keystore directories passed to sbkeysync via
// the SignatureDbUpdateKeystores field of the params argument. This function assumes that sbkeysync is executed with the
// "--no-default-keystores" option. When there are pending updates in the specified directories, this function will generate a PCR
// policy that is compatible with the current database contents and the database contents computed for each individual update.
// Note that sbkeysync ignores errors when applying updates - if any of the pending updates don't apply for some reason, the generated
// PCR profile will be invalid.
//
// For the most common case where there are no signature database updates pending in the specified keystore directories and each image
// load event sequence corresponds to loads of images that are all verified with the same chain of trust, this is a complicated way of
// adding a single PCR digest to the provided secboot.PCRProtectionProfile.
func AddSecureBootPolicyProfile(profile *secboot_tpm2.PCRProtectionProfile, params *SecureBootPolicyProfileParams) error {
	env := params.Environment
	if env == nil {
		env = defaultEnv
	}

	// Load event log
	log, err := env.ReadEventLog()
	if err != nil {
		return xerrors.Errorf("cannot parse TCG event log: %w", err)
	}

	if !log.Algorithms.Contains(params.PCRAlgorithm) {
		return errors.New("cannot compute secure boot policy profile: the TCG event log does not have the requested algorithm")
	}

	// Make sure that the current boot is sane.
	seenSecureBootConfig := false
	for _, event := range log.Events {
		switch event.PCRIndex {
		case bootManagerCodePCR:
			if event.EventType == tcglog.EventTypeEFIAction && event.Data == tcglog.EFIReturningFromEFIApplicationEvent {
				// Firmware should record this event if an EFI application returns to the boot manager. Bail out if this happened because the policy might not make sense.
				return errors.New("cannot compute secure boot policy profile: the current boot was preceeded by a boot attempt to an EFI " +
					"application that returned to the boot manager, without a reboot in between")
			}
		case secureBootPCR:
			switch event.EventType {
			case tcglog.EventTypeEFIVariableDriverConfig:
				if err, isErr := event.Data.(error); isErr {
					return fmt.Errorf("%s secure boot policy event has invalid event data: %v", event.EventType, err)
				}
				efiVarData := event.Data.(*tcglog.EFIVariableData)
				if efiVarData.VariableName == efi.GlobalVariable && efiVarData.UnicodeName == sbStateName {
					switch {
					case seenSecureBootConfig:
						// The spec says that secure boot policy must be measured again if the system supports changing it before ExitBootServices
						// without a reboot. But the policy we create won't make sense, so bail out
						return errors.New("cannot compute secure boot policy profile: secure boot configuration was modified after the initial " +
							"configuration was measured, without performing a reboot")
					case efiVarData.VariableData[0] == 0x00:
						return errors.New("cannot compute secure boot policy profile: the current boot was performed with secure boot disabled in firmware")
					}
					seenSecureBootConfig = true
				}
			case tcglog.EventTypeEFIVariableAuthority:
				if err, isErr := event.Data.(error); isErr {
					return fmt.Errorf("%s secure boot policy event has invalid event data: %v", event.EventType, err)
				}
				efiVarData := event.Data.(*tcglog.EFIVariableData)
				if efiVarData.VariableName == shimGuid && efiVarData.UnicodeName == mokSbStateName {
					// MokSBState is set to 0x01 if secure boot enforcement is disabled in shim. The variable is deleted when secure boot enforcement
					// is enabled, so don't bother looking at the value here. It doesn't make a lot of sense to create a policy if secure boot
					// enforcement is disabled in shim
					return errors.New("cannot compute secure boot policy profile: the current boot was performed with validation disabled in Shim")
				}
			}
		}
	}

	// Initialize the secure boot PCR to 0
	profile.AddPCRValue(params.PCRAlgorithm, secureBootPCR, make(tpm2.Digest, params.PCRAlgorithm.Size()))

	// Compute a list of pending EFI signature DB updates.
	sigDbUpdates, err := buildSignatureDbUpdateList(params.SignatureDbUpdateKeystores)
	if err != nil {
		return xerrors.Errorf("cannot build list of UEFI signature DB updates: %w", err)
	}

	gen := &secureBootPolicyGen{params.PCRAlgorithm, env, params.LoadSequences, log.Events, sigDbUpdates}

	profile1 := secboot_tpm2.NewPCRProtectionProfile()
	if err := gen.run(profile1, sigDbUpdateQuirkModeNone); err != nil {
		return xerrors.Errorf("cannot compute secure boot policy profile: %w", err)
	}

	profile2 := secboot_tpm2.NewPCRProtectionProfile()
	if err := gen.run(profile2, sigDbUpdateQuirkModeDedupIgnoresOwner); err != nil {
		return xerrors.Errorf("cannot compute secure boot policy profile: %w", err)
	}

	profile.AddProfileOR(profile1, profile2)
	return nil
}
