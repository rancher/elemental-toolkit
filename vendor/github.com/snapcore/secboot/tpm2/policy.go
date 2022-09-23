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
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/util"

	"golang.org/x/xerrors"
)

var (
	// lockNVIndex1Attrs are the attributes for the first global lock NV index.
	lockNVIndex1Attrs = tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVPolicyWrite | tpm2.AttrNVAuthRead | tpm2.AttrNVNoDA | tpm2.AttrNVReadStClear)
)

// dynamicPolicyComputeParams provides the parameters to computeDynamicPolicy.
type dynamicPolicyComputeParams struct {
	key crypto.PrivateKey // Key used to authorize the generated dynamic authorization policy

	// signAlg is the digest algorithm for the signature used to authorize the generated dynamic authorization policy. It must
	// match the name algorithm of the public part of key that will be loaded in to the TPM for verification.
	signAlg           tpm2.HashAlgorithmId
	pcrs              tpm2.PCRSelectionList // PCR selection
	pcrDigests        tpm2.DigestList       // Approved PCR digests
	policyCounterName tpm2.Name             // Name of the NV index used for revoking authorization policies
	policyCount       uint64                // Count for this policy, used for revocation
}

// policyOrDataNode represents a collection of up to 8 digests used in a single TPM2_PolicyOR invocation, and forms part of a tree
// of nodes in order to support authorization policies with more than 8 conditions.
type policyOrDataNode struct {
	Next    uint32 // Index of the parent node in the containing slice, relative to this node. Zero indicates that this is the root node
	Digests tpm2.DigestList
}

type policyOrDataTree []policyOrDataNode

// dynamicPolicyData is an output of computeDynamicPolicy and provides metadata for executing a policy session.
type dynamicPolicyData struct {
	pcrSelection              tpm2.PCRSelectionList
	pcrOrData                 policyOrDataTree
	policyCount               uint64
	authorizedPolicy          tpm2.Digest
	authorizedPolicySignature *tpm2.Signature
}

// dynamicPolicyDataRaw_v0 is version 0 of the on-disk format of dynamicPolicyData.
type dynamicPolicyDataRaw_v0 struct {
	PCRSelection              tpm2.PCRSelectionList
	PCROrData                 policyOrDataTree
	PolicyCount               uint64
	AuthorizedPolicy          tpm2.Digest
	AuthorizedPolicySignature *tpm2.Signature
}

func (d *dynamicPolicyDataRaw_v0) data() *dynamicPolicyData {
	return &dynamicPolicyData{
		pcrSelection:              d.PCRSelection,
		pcrOrData:                 d.PCROrData,
		policyCount:               d.PolicyCount,
		authorizedPolicy:          d.AuthorizedPolicy,
		authorizedPolicySignature: d.AuthorizedPolicySignature}
}

// makeDynamicPolicyDataRaw_v0 converts dynamicPolicyData to version 0 of the on-disk format.
func makeDynamicPolicyDataRaw_v0(data *dynamicPolicyData) *dynamicPolicyDataRaw_v0 {
	return &dynamicPolicyDataRaw_v0{
		PCRSelection:              data.pcrSelection,
		PCROrData:                 data.pcrOrData,
		PolicyCount:               data.policyCount,
		AuthorizedPolicy:          data.authorizedPolicy,
		AuthorizedPolicySignature: data.authorizedPolicySignature}
}

// staticPolicyComputeParams provides the parameters to computeStaticPolicy.
type staticPolicyComputeParams struct {
	key                 *tpm2.Public   // Public part of key used to authorize a dynamic authorization policy
	pcrPolicyCounterPub *tpm2.NVPublic // Public area of the NV counter used for revoking PCR policies
}

// staticPolicyData is an output of computeStaticPolicy and provides metadata for executing a policy session.
type staticPolicyData struct {
	authPublicKey          *tpm2.Public
	pcrPolicyCounterHandle tpm2.Handle
	v0PinIndexAuthPolicies tpm2.DigestList
}

// staticPolicyDataRaw_v0 is version 0 of the on-disk format of staticPolicyData.
type staticPolicyDataRaw_v0 struct {
	AuthPublicKey        *tpm2.Public
	PinIndexHandle       tpm2.Handle
	PinIndexAuthPolicies tpm2.DigestList
}

func (d *staticPolicyDataRaw_v0) data() *staticPolicyData {
	return &staticPolicyData{
		authPublicKey:          d.AuthPublicKey,
		pcrPolicyCounterHandle: d.PinIndexHandle,
		v0PinIndexAuthPolicies: d.PinIndexAuthPolicies}
}

// makeStaticPolicyDataRaw_v0 converts staticPolicyData to version 0 of the on-disk format.
func makeStaticPolicyDataRaw_v0(data *staticPolicyData) *staticPolicyDataRaw_v0 {
	return &staticPolicyDataRaw_v0{
		AuthPublicKey:        data.authPublicKey,
		PinIndexHandle:       data.pcrPolicyCounterHandle,
		PinIndexAuthPolicies: data.v0PinIndexAuthPolicies}
}

// staticPolicyDataRaw_v1 is version 1 of the on-disk format of staticPolicyData.
type staticPolicyDataRaw_v1 struct {
	AuthPublicKey          *tpm2.Public
	PCRPolicyCounterHandle tpm2.Handle
	PCRPolicyRef           tpm2.Nonce
}

func (d *staticPolicyDataRaw_v1) data() *staticPolicyData {
	return &staticPolicyData{
		authPublicKey:          d.AuthPublicKey,
		pcrPolicyCounterHandle: d.PCRPolicyCounterHandle}
}

// makeStaticPolicyDataRaw_v1 converts staticPolicyData to version 1 of the on-disk format.
func makeStaticPolicyDataRaw_v1(data *staticPolicyData) *staticPolicyDataRaw_v1 {
	return &staticPolicyDataRaw_v1{
		AuthPublicKey:          data.authPublicKey,
		PCRPolicyCounterHandle: data.pcrPolicyCounterHandle}
}

// computeV0PinNVIndexPostInitAuthPolicies computes the authorization policy digests associated with the post-initialization
// actions on a NV index created with the removed createPinNVIndex for version 0 key files. These are:
// - A policy for updating the index to revoke old dynamic authorization policies, requiring an assertion signed by the key
//   associated with updateKeyName.
// - A policy for updating the authorization value (PIN / passphrase), requiring knowledge of the current authorization value.
// - A policy for reading the counter value without knowing the authorization value, as the value isn't secret.
// - A policy for using the counter value in a TPM2_PolicyNV assertion without knowing the authorization value.
func computeV0PinNVIndexPostInitAuthPolicies(alg tpm2.HashAlgorithmId, updateKeyName tpm2.Name) tpm2.DigestList {
	var out tpm2.DigestList
	// Compute a policy for incrementing the index to revoke dynamic authorization policies, requiring an assertion signed by the
	// key associated with updateKeyName.
	trial := util.ComputeAuthPolicy(alg)
	trial.PolicyCommandCode(tpm2.CommandNVIncrement)
	trial.PolicyNvWritten(true)
	trial.PolicySigned(updateKeyName, nil)
	out = append(out, trial.GetDigest())

	// Compute a policy for updating the authorization value of the index, requiring knowledge of the current authorization value.
	trial = util.ComputeAuthPolicy(alg)
	trial.PolicyCommandCode(tpm2.CommandNVChangeAuth)
	trial.PolicyAuthValue()
	out = append(out, trial.GetDigest())

	// Compute a policy for reading the counter value without knowing the authorization value.
	trial = util.ComputeAuthPolicy(alg)
	trial.PolicyCommandCode(tpm2.CommandNVRead)
	out = append(out, trial.GetDigest())

	// Compute a policy for using the counter value in a TPM2_PolicyNV assertion without knowing the authorization value.
	trial = util.ComputeAuthPolicy(alg)
	trial.PolicyCommandCode(tpm2.CommandPolicyNV)
	out = append(out, trial.GetDigest())

	return out
}

// pcrPolicyCounterHandle abstracts access to the PCR policy counter in order to
// support the current style of index created with createPcrPolicyCounter, and the
// legacy PIN index originally created by (the now deleted) createPinNVINdex.
type pcrPolicyCounterHandle interface {
	// Get returns the current value of the associated NV counter index.
	Get(tpm *tpm2.TPMContext, session tpm2.SessionContext) (uint64, error)

	// Incremement will increment the associated NV counter index by one.
	// This requires a signed authorization.
	Increment(tpm *tpm2.TPMContext, key crypto.PrivateKey, session tpm2.SessionContext) error
}

func incrementPcrPolicyCounterTo(tpm *tpm2.TPMContext, handle pcrPolicyCounterHandle, value uint64,
	key crypto.PrivateKey, session tpm2.SessionContext) error {
	for {
		current, err := handle.Get(tpm, session)
		switch {
		case err != nil:
			return xerrors.Errorf("cannot read current value: %w", err)
		case current > value:
			return errors.New("cannot set counter to a lower value")
		}

		if current == value {
			return nil
		}

		if err := handle.Increment(tpm, key, session); err != nil {
			return xerrors.Errorf("cannot increment counter: %w", err)
		}
	}
}

type pcrPolicyCounterCommon struct {
	pub          *tpm2.NVPublic
	updateKey    *tpm2.Public
	authPolicies tpm2.DigestList
}

type pcrPolicyCounterV0 struct {
	pcrPolicyCounterCommon
}

func (c *pcrPolicyCounterV0) Get(tpm *tpm2.TPMContext, session tpm2.SessionContext) (uint64, error) {
	authSession, err := tpm.StartAuthSession(nil, nil, tpm2.SessionTypePolicy, nil, c.pub.NameAlg)
	if err != nil {
		return 0, xerrors.Errorf("cannot begin policy session: %w", err)
	}
	defer tpm.FlushContext(authSession)

	// See the comment for computeV0PinNVIndexPostInitAuthPolicies for a description of the authorization policy
	// for the v0 NV index. Because the v0 NV index was also used for the PIN, it needed an authorization policy
	// to permit reading the counter value without knowing the authorization value of the index.
	if err := tpm.PolicyCommandCode(authSession, tpm2.CommandNVRead); err != nil {
		return 0, xerrors.Errorf("cannot execute assertion to read counter: %w", err)
	}
	if err := tpm.PolicyOR(authSession, c.authPolicies); err != nil {
		return 0, xerrors.Errorf("cannot execute assertion to increment counter: %w", err)
	}

	index, err := tpm2.CreateNVIndexResourceContextFromPublic(c.pub)
	if err != nil {
		return 0, xerrors.Errorf("cannot create context for NV index: %w", err)
	}

	value, err := tpm.NVReadCounter(index, index, authSession, session.IncludeAttrs(tpm2.AttrAudit))
	if err != nil {
		return 0, xerrors.Errorf("cannot read counter: %w", err)
	}

	return value, nil
}

func (c *pcrPolicyCounterV0) Increment(tpm *tpm2.TPMContext, key crypto.PrivateKey, session tpm2.SessionContext) error {
	index, err := tpm2.CreateNVIndexResourceContextFromPublic(c.pub)
	if err != nil {
		return xerrors.Errorf("cannot create context for NV index: %w", err)
	}

	// Begin a policy session to increment the index.
	policySession, err := tpm.StartAuthSession(nil, nil, tpm2.SessionTypePolicy, nil, c.pub.NameAlg)
	if err != nil {
		return xerrors.Errorf("cannot begin policy session: %w", err)
	}
	defer tpm.FlushContext(policySession)

	// Load the public part of the key in to the TPM. There's no integrity protection for this command as if it's altered in
	// transit then either the signature verification fails or the policy digest will not match the one associated with the NV
	// index.
	keyLoaded, err := tpm.LoadExternal(nil, c.updateKey, tpm2.HandleEndorsement)
	if err != nil {
		return xerrors.Errorf("cannot load public part of key used to verify authorization signature: %w", err)
	}
	defer tpm.FlushContext(keyLoaded)

	// Create a signed authorization. keyData.validate checks that this scheme is compatible with the key
	scheme := tpm2.SigScheme{
		Scheme: tpm2.SigSchemeAlgRSAPSS,
		Details: &tpm2.SigSchemeU{
			RSAPSS: &tpm2.SigSchemeRSAPSS{
				HashAlg: c.updateKey.NameAlg}}}
	signature, err := util.SignPolicyAuthorization(key, &scheme, policySession.NonceTPM(), nil, nil, 0)
	if err != nil {
		return xerrors.Errorf("cannot sign authorization: %w", err)
	}

	// See the comment for computeV0PinNVIndexPostInitAuthPolicies for a description of the authorization policy
	// for the v0 NV index.
	if err := tpm.PolicyCommandCode(policySession, tpm2.CommandNVIncrement); err != nil {
		return xerrors.Errorf("cannot execute assertion to increment counter: %w", err)
	}
	if err := tpm.PolicyNvWritten(policySession, true); err != nil {
		return xerrors.Errorf("cannot execute assertion to increment counter: %w", err)
	}
	if _, _, err := tpm.PolicySigned(keyLoaded, policySession, true, nil, nil, 0, signature); err != nil {
		return xerrors.Errorf("cannot execute assertion to increment counter: %w", err)
	}
	if err := tpm.PolicyOR(policySession, c.authPolicies); err != nil {
		return xerrors.Errorf("cannot execute assertion to increment counter: %w", err)
	}

	// Increment the index.
	if err := tpm.NVIncrement(index, index, policySession, session.IncludeAttrs(tpm2.AttrAudit)); err != nil {
		return xerrors.Errorf("cannot increment NV index: %w", err)
	}

	return nil
}

// newPcrPolicyCounterHandleV0 creates a handle to perform operations on a legacy V0 PIN NV index,
// originally created with createPinNVIndex (which has since been deleted). The key originally passed
// to createPinNVIndex must be supplied via the updateKey argument, and the authorization
// policy digests returned from createPinNVIndex must be supplied via the authPolicies argument.
func newPcrPolicyCounterHandleV0(pub *tpm2.NVPublic, updateKey *tpm2.Public, authPolicies tpm2.DigestList) pcrPolicyCounterHandle {
	return &pcrPolicyCounterV0{pcrPolicyCounterCommon{pub: pub, updateKey: updateKey, authPolicies: authPolicies}}
}

type pcrPolicyCounterV1 struct {
	pcrPolicyCounterCommon
}

func (c *pcrPolicyCounterV1) Get(tpm *tpm2.TPMContext, session tpm2.SessionContext) (uint64, error) {
	index, err := tpm2.CreateNVIndexResourceContextFromPublic(c.pub)
	if err != nil {
		return 0, xerrors.Errorf("cannot create context for NV index: %w", err)
	}

	value, err := tpm.NVReadCounter(index, index, nil, session.IncludeAttrs(tpm2.AttrAudit))
	if err != nil {
		return 0, xerrors.Errorf("cannot read counter: %w", err)
	}

	return value, nil
}

func (c *pcrPolicyCounterV1) Increment(tpm *tpm2.TPMContext, key crypto.PrivateKey, session tpm2.SessionContext) error {
	index, err := tpm2.CreateNVIndexResourceContextFromPublic(c.pub)
	if err != nil {
		return xerrors.Errorf("cannot create context for NV index: %w", err)
	}

	// Begin a policy session to increment the index.
	policySession, err := tpm.StartAuthSession(nil, nil, tpm2.SessionTypePolicy, nil, c.pub.NameAlg)
	if err != nil {
		return xerrors.Errorf("cannot begin policy session: %w", err)
	}
	defer tpm.FlushContext(policySession)

	// Load the public part of the key in to the TPM. There's no integrity protection for this command as if it's altered in
	// transit then either the signature verification fails or the policy digest will not match the one associated with the NV
	// index.
	keyLoaded, err := tpm.LoadExternal(nil, c.updateKey, tpm2.HandleEndorsement)
	if err != nil {
		return xerrors.Errorf("cannot load public part of key used to verify authorization signature: %w", err)
	}
	defer tpm.FlushContext(keyLoaded)

	// Create a signed authorization. keyData.validate checks that this scheme is compatible with the key
	scheme := tpm2.SigScheme{
		Scheme: tpm2.SigSchemeAlgECDSA,
		Details: &tpm2.SigSchemeU{
			ECDSA: &tpm2.SigSchemeECDSA{
				HashAlg: c.updateKey.NameAlg}}}
	signature, err := util.SignPolicyAuthorization(key, &scheme, policySession.NonceTPM(), nil, nil, 0)
	if err != nil {
		return xerrors.Errorf("cannot sign authorization: %w", err)
	}

	if _, _, err := tpm.PolicySigned(keyLoaded, policySession, true, nil, nil, 0, signature); err != nil {
		return xerrors.Errorf("cannot execute assertion to increment counter: %w", err)
	}
	if err := tpm.PolicyOR(policySession, c.authPolicies); err != nil {
		return xerrors.Errorf("cannot execute assertion to increment counter: %w", err)
	}

	// Increment the index.
	if err := tpm.NVIncrement(index, index, policySession, session.IncludeAttrs(tpm2.AttrAudit)); err != nil {
		return xerrors.Errorf("cannot increment NV index: %w", err)
	}

	return nil
}

// newPcrPolicyCounterHandleV1 creates a handle to perform operations on a V1 PCR policy counter
// NV index, created with createPcrPolicyCounter. The key passed to createPcrPolicyCounter must
// be supplied via the updateKey argument.
func newPcrPolicyCounterHandleV1(pub *tpm2.NVPublic, updateKey *tpm2.Public) (pcrPolicyCounterHandle, error) {
	updateKeyName, err := updateKey.Name()
	if err != nil {
		return nil, xerrors.Errorf("cannot compute name of update key: %w", err)
	}

	authPolicies := computePcrPolicyCounterAuthPolicies(pub.NameAlg, updateKeyName)
	return &pcrPolicyCounterV1{pcrPolicyCounterCommon{pub: pub, updateKey: updateKey, authPolicies: authPolicies}}, nil
}

// computePcrPolicyCounterAuthPolicies computes the authorization policy digests passed to TPM2_PolicyOR for a PCR
// policy counter that can be updated with the key associated with updateKeyName.
func computePcrPolicyCounterAuthPolicies(alg tpm2.HashAlgorithmId, updateKeyName tpm2.Name) tpm2.DigestList {
	// The NV index requires 2 policies:
	// - A policy to initialize the index with no authorization
	// - A policy for updating the index to revoke old PCR policies using a signed assertion. This isn't done for security
	//   reasons, but just to make it harder to accidentally increment the counter for anyone interacting with the TPM.
	// This is simpler than the policy required for the v0 PIN NV index because it doesn't require additional authorization
	// policy branches to allow its authorization value to be changed, or to be able to read the counter value or use it in
	// a policy assertion without knowing the authorization value (reading the value of this counter does require the
	// authorization value, but it is always empty and this policy doesn't allow it to be changed).
	var authPolicies tpm2.DigestList

	trial := util.ComputeAuthPolicy(alg)
	trial.PolicyNvWritten(false)
	authPolicies = append(authPolicies, trial.GetDigest())

	trial = util.ComputeAuthPolicy(alg)
	trial.PolicySigned(updateKeyName, nil)
	authPolicies = append(authPolicies, trial.GetDigest())

	return authPolicies
}

// createPcrPolicyCounter creates and initializes a NV counter that is associated with a sealed key object and is used for
// implementing dynamic authorization policy revocation.
//
// The NV index will be created with attributes that allow anyone to read the index, and an authorization policy that permits
// TPM2_NV_Increment with a signed authorization policy.
func createPcrPolicyCounter(tpm *tpm2.TPMContext, handle tpm2.Handle, updateKey *tpm2.Public, hmacSession tpm2.SessionContext) (*tpm2.NVPublic, uint64, error) {
	nameAlg := tpm2.HashAlgorithmSHA256

	updateKeyName, err := updateKey.Name()
	if err != nil {
		return nil, 0, xerrors.Errorf("cannot compute name of update key: %w", err)
	}

	authPolicies := computePcrPolicyCounterAuthPolicies(nameAlg, updateKeyName)

	trial := util.ComputeAuthPolicy(nameAlg)
	trial.PolicyOR(authPolicies)

	// Define the NV index
	public := &tpm2.NVPublic{
		Index:      handle,
		NameAlg:    nameAlg,
		Attrs:      tpm2.NVTypeCounter.WithAttrs(tpm2.AttrNVPolicyWrite | tpm2.AttrNVAuthRead | tpm2.AttrNVNoDA),
		AuthPolicy: trial.GetDigest(),
		Size:       8}

	index, err := tpm.NVDefineSpace(tpm.OwnerHandleContext(), nil, public, hmacSession)
	if err != nil {
		return nil, 0, xerrors.Errorf("cannot define NV space: %w", err)
	}

	// NVDefineSpace was integrity protected, so we know that we have an index with the expected public area at the handle we specified
	// at this point.

	succeeded := false
	defer func() {
		if succeeded {
			return
		}
		tpm.NVUndefineSpace(tpm.OwnerHandleContext(), index, hmacSession)
	}()

	// Begin a session to initialize the index.
	policySession, err := tpm.StartAuthSession(nil, nil, tpm2.SessionTypePolicy, nil, nameAlg)
	if err != nil {
		return nil, 0, xerrors.Errorf("cannot begin policy session to initialize NV index: %w", err)
	}
	defer tpm.FlushContext(policySession)

	// Execute the policy assertions
	if err := tpm.PolicyNvWritten(policySession, false); err != nil {
		return nil, 0, xerrors.Errorf("cannot execute assertion to initialize NV index: %w", err)
	}
	if err := tpm.PolicyOR(policySession, authPolicies); err != nil {
		return nil, 0, xerrors.Errorf("cannot execute assertion to initialize NV index: %w", err)
	}

	// Initialize the index
	if err := tpm.NVIncrement(index, index, policySession, hmacSession.IncludeAttrs(tpm2.AttrAudit)); err != nil {
		return nil, 0, xerrors.Errorf("cannot initialize NV index: %w", err)
	}

	// The index has a different name now that it has been written, so update the public area we return so that it can be used
	// to construct an authorization policy.
	public.Attrs |= tpm2.AttrNVWritten

	h, err := newPcrPolicyCounterHandleV1(public, updateKey)
	if err != nil {
		panic(fmt.Sprintf("cannot create handle to read counter value: %v", err))
	}

	value, err := h.Get(tpm, hmacSession)
	if err != nil {
		return nil, 0, xerrors.Errorf("cannot read current counter value: %w", err)
	}

	succeeded = true
	return public, value, nil
}

// ensureSufficientORDigests turns a single digest in to a pair of identical digests. This is because TPM2_PolicyOR assertions
// require more than one digest. This avoids having a separate policy sequence when there is only a single digest, without having
// to store duplicate digests on disk.
func ensureSufficientORDigests(digests tpm2.DigestList) tpm2.DigestList {
	if len(digests) == 1 {
		return tpm2.DigestList{digests[0], digests[0]}
	}
	return digests
}

// computePcrPolicyRefFromCounterName computes the reference used for authorization of signed PCR policies from the supplied
// PCR policy counter name. If name is empty, then the name of the null handle is assumed. The policy ref serves 2 purposes:
// 1) It limits the scope of the signed policy to just PCR policies (the dynamic authorization policy key may be able to sign
//    different types of policy in the future, for example, to permit recovery with a signed assertion.
// 2) It binds the name of the PCR policy counter to the static authorization policy.
func computePcrPolicyRefFromCounterName(name tpm2.Name) tpm2.Nonce {
	if len(name) == 0 {
		name = make(tpm2.Name, binary.Size(tpm2.Handle(0)))
		binary.BigEndian.PutUint32(name, uint32(tpm2.HandleNull))
	}

	h := tpm2.HashAlgorithmSHA256.NewHash()
	h.Write([]byte("AUTH-PCR-POLICY"))
	h.Write(name)

	return h.Sum(nil)
}

// computePcrPolicyRefFromCounterContext computes the reference used for authorization of signed PCR policies from the supplied
// ResourceContext.
func computePcrPolicyRefFromCounterContext(context tpm2.ResourceContext) tpm2.Nonce {
	var name tpm2.Name
	if context != nil {
		name = context.Name()
	}

	return computePcrPolicyRefFromCounterName(name)
}

// computeStaticPolicy computes the part of an authorization policy that is bound to a sealed key object and never changes. The
// static policy asserts that the following are true:
// - The signed PCR policy created by computeDynamicPolicy is valid and has been satisfied (by way of a PolicyAuthorize assertion,
//   which allows the PCR policy to be updated without creating a new sealed key object).
// - Knowledge of the the authorization value for the entity on which the policy session is used has been demonstrated by the
//   caller - this will be used in the future as part of the passphrase integration.
func computeStaticPolicy(alg tpm2.HashAlgorithmId, input *staticPolicyComputeParams) (*staticPolicyData, tpm2.Digest, error) {
	keyName, err := input.key.Name()
	if err != nil {
		return nil, nil, xerrors.Errorf("cannot compute name of signing key for dynamic policy authorization: %w", err)
	}

	pcrPolicyCounterHandle := tpm2.HandleNull
	var pcrPolicyCounterName tpm2.Name
	if input.pcrPolicyCounterPub != nil {
		pcrPolicyCounterHandle = input.pcrPolicyCounterPub.Index
		pcrPolicyCounterName, err = input.pcrPolicyCounterPub.Name()
		if err != nil {
			return nil, nil, xerrors.Errorf("cannot compute name of PCR policy counter: %w", err)
		}
	}

	trial := util.ComputeAuthPolicy(alg)
	trial.PolicyAuthorize(computePcrPolicyRefFromCounterName(pcrPolicyCounterName), keyName)
	trial.PolicyAuthValue()

	return &staticPolicyData{
		authPublicKey:          input.key,
		pcrPolicyCounterHandle: pcrPolicyCounterHandle}, trial.GetDigest(), nil
}

// computePolicyORData computes data required to perform a sequence of TPM2_PolicyOR assertions in order to support compound
// authorization policies with more than 8 conditions (which is the limit of the TPM). Its main purpose is to support PCR policies
// with more than 8 conditions. It works by turning a list of digests (or, conditions) in to a tree of nodes, with each node
// containing no more than 8 digests that can be used in a single TPM2_PolicyOR assertion, the root of the tree containing digests
// for the final TPM2_PolicyOR assertion, and leaf nodes containing digests for each OR condition. Whilst the returned data is
// conceptually a tree, the layout in memory is just a slice of tables of up to 8 digests, each with an index that enables the code
// executing the assertions to traverse upwards through the tree by just advancing to another entry in the slice. This format is
// easily serialized. After the computations are completed, the provided *util.TrialAuthPolicy will be updated.
//
// The returned data is used by firstly finding the leaf node which contains the current session digest. Once this is found, a
// TPM2_PolicyOR assertion is executed on the digests in that node, and then the tree is traversed upwards to the root node, executing
// TPM2_PolicyOR assertions along the way - see executePolicyORAssertions.
func computePolicyORData(alg tpm2.HashAlgorithmId, trial *util.TrialAuthPolicy, digests tpm2.DigestList) policyOrDataTree {
	var data policyOrDataTree
	curNode := 0
	var nextDigests tpm2.DigestList

	for {
		n := len(digests)
		if n > 8 {
			// The TPM only supports 8 conditions in TPM2_PolicyOR.
			n = 8
		}

		data = append(data, policyOrDataNode{Digests: digests[:n]})
		if n == len(digests) && len(nextDigests) == 0 {
			// All of the digests at this level fit in to a single TPM2_PolicyOR command, so this becomes the root node.
			break
		}

		// Consume the next n digests to fit in to this node and produce a single digest that will go in to the parent node.
		trial := util.ComputeAuthPolicy(alg)
		trial.PolicyOR(ensureSufficientORDigests(digests[:n]))
		nextDigests = append(nextDigests, trial.GetDigest())

		// We've consumed n digests, so adjust the slice to point to the next ones to consume to produce a sibling node.
		digests = digests[n:]

		if len(digests) == 0 {
			// There are no digests left to produce sibling nodes, and we have a collection of digests to produce parent nodes. Update the
			// nodes produced at this level to point to the parent nodes we're going to produce on the subsequent iterations.
			for i := range nextDigests {
				// At this point, len(nextDigests) == (len(data) - curNode).
				// 'len(nextDigests) - i' initializes Next to point to the end of data (ie, data[len(data)]), and the '+ (i / 8)' advances it to
				// point to the parent node that will be created on subsequent iterations, taking in to account that each node will have up to
				// 8 child nodes.
				data[curNode+i].Next = uint32(len(nextDigests) - i + (i / 8))
			}
			// Grab the digests produced for the nodes at this level to produce the parent nodes.
			curNode += len(nextDigests)
			digests = nextDigests
			nextDigests = nil
		}
	}

	trial.PolicyOR(ensureSufficientORDigests(digests))
	return data
}

// computeDynamicPolicy computes the PCR policy associated with a sealed key object, and can be updated without having to create a
// new sealed key object as it takes advantage of the PolicyAuthorize assertion. The PCR policy asserts that the following are true:
// - The selected PCRs contain expected values - ie, one of the sets of permitted values specified by the caller to this function,
//   indicating that the device is in an expected state. This is done by a single PolicyPCR assertion and then one or more PolicyOR
//   assertions (depending on how many sets of permitted PCR values there are).
// - The PCR policy hasn't been revoked. This is done using a PolicyNV assertion to assert that the value of an optional NV counter
//   is not greater than the expected value.
// The computed PCR policy digest is signed with the supplied asymmetric key, and the signature of this is validated before executing
// the corresponding PolicyAuthorize assertion as part of the static policy.
func computeDynamicPolicy(version uint32, alg tpm2.HashAlgorithmId, input *dynamicPolicyComputeParams) (*dynamicPolicyData, error) {
	if len(input.pcrDigests) == 0 {
		return nil, errors.New("no PCR digests specified")
	}

	// Compute the policy digest that would result from a TPM2_PolicyPCR assertion for each condition
	var pcrOrDigests tpm2.DigestList
	for _, d := range input.pcrDigests {
		trial := util.ComputeAuthPolicy(alg)
		trial.PolicyPCR(d, input.pcrs)
		pcrOrDigests = append(pcrOrDigests, trial.GetDigest())
	}

	trial := util.ComputeAuthPolicy(alg)
	pcrOrData := computePolicyORData(alg, trial, pcrOrDigests)

	if len(input.policyCounterName) > 0 {
		operandB := make([]byte, 8)
		binary.BigEndian.PutUint64(operandB, input.policyCount)
		trial.PolicyNV(input.policyCounterName, operandB, 0, tpm2.OpUnsignedLE)
	}

	authorizedPolicy := trial.GetDigest()

	// Create a digest to sign
	h := input.signAlg.NewHash()
	h.Write(authorizedPolicy)
	if version > 0 {
		h.Write(computePcrPolicyRefFromCounterName(input.policyCounterName))
	}

	// Sign the digest
	var signature tpm2.Signature
	if version == 0 {
		sig, err := rsa.SignPSS(rand.Reader, input.key.(*rsa.PrivateKey), input.signAlg.GetHash(), h.Sum(nil),
			&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		if err != nil {
			return nil, xerrors.Errorf("cannot provide signature for initializing NV index: %w", err)
		}

		signature = tpm2.Signature{
			SigAlg: tpm2.SigSchemeAlgRSAPSS,
			Signature: &tpm2.SignatureU{
				RSAPSS: &tpm2.SignatureRSAPSS{
					Hash: input.signAlg,
					Sig:  tpm2.PublicKeyRSA(sig)}}}
	} else {
		sigR, sigS, err := ecdsa.Sign(rand.Reader, input.key.(*ecdsa.PrivateKey), h.Sum(nil))
		if err != nil {
			return nil, xerrors.Errorf("cannot provide signature for initializing NV index: %w", err)
		}

		signature = tpm2.Signature{
			SigAlg: tpm2.SigSchemeAlgECDSA,
			Signature: &tpm2.SignatureU{
				ECDSA: &tpm2.SignatureECDSA{
					Hash:       input.signAlg,
					SignatureR: sigR.Bytes(),
					SignatureS: sigS.Bytes()}}}
	}

	return &dynamicPolicyData{
		pcrSelection:              input.pcrs,
		pcrOrData:                 pcrOrData,
		policyCount:               input.policyCount,
		authorizedPolicy:          authorizedPolicy,
		authorizedPolicySignature: &signature}, nil
}

type staticPolicyDataError struct {
	err error
}

func (e staticPolicyDataError) Error() string {
	return e.err.Error()
}

func (e staticPolicyDataError) Unwrap() error {
	return e.err
}

func isStaticPolicyDataError(err error) bool {
	var e staticPolicyDataError
	return xerrors.As(err, &e)
}

type dynamicPolicyDataError struct {
	err error
}

func (e dynamicPolicyDataError) Error() string {
	return e.err.Error()
}

func (e dynamicPolicyDataError) Unwrap() error {
	return e.err
}

func isDynamicPolicyDataError(err error) bool {
	var e dynamicPolicyDataError
	return xerrors.As(err, &e)
}

// executePolicyORAssertions takes the data produced by computePolicyORData and executes a sequence of TPM2_PolicyOR assertions, in
// order to support compound policies with more than 8 conditions.
func executePolicyORAssertions(tpm *tpm2.TPMContext, session tpm2.SessionContext, data policyOrDataTree) error {
	// First of all, obtain the current digest of the session.
	currentDigest, err := tpm.PolicyGetDigest(session)
	if err != nil {
		return xerrors.Errorf("cannot obtain current session digest: %w", err)
	}

	if len(data) == 0 {
		return errors.New("no policy data")
	}

	// Find the leaf node that contains the current digest of the session.
	index := -1
	end := data[0].Next
	if end == 0 {
		end = 1
	}

	for i := 0; i < len(data) && i < int(end); i++ {
		if digestListContains(data[i].Digests, currentDigest) {
			// We've got a match!
			index = i
			break
		}
	}
	if index == -1 {
		return errors.New("current session digest not found in policy data")
	}

	// Execute a TPM2_PolicyOR assertion on the digests in the leaf node and then traverse up the tree to the root node, executing
	// TPM2_PolicyOR assertions along the way.
	for lastIndex := -1; index > lastIndex && index < len(data); index += int(data[index].Next) {
		lastIndex = index
		if err := tpm.PolicyOR(session, ensureSufficientORDigests(data[index].Digests)); err != nil {
			return err
		}
		if data[index].Next == 0 {
			// This is the root node, so we're finished.
			break
		}
	}
	return nil
}

// executePolicySession executes an authorization policy session using the supplied metadata. On success, the supplied policy
// session can be used for authorization.
func executePolicySession(tpm *tpm2.TPMContext, policySession tpm2.SessionContext, version uint32, staticInput *staticPolicyData,
	dynamicInput *dynamicPolicyData, hmacSession tpm2.SessionContext) error {
	if err := tpm.PolicyPCR(policySession, nil, dynamicInput.pcrSelection); err != nil {
		return xerrors.Errorf("cannot execute PCR assertion: %w", err)
	}

	if err := executePolicyORAssertions(tpm, policySession, dynamicInput.pcrOrData); err != nil {
		switch {
		case tpm2.IsTPMError(err, tpm2.AnyErrorCode, tpm2.CommandPolicyGetDigest):
			return xerrors.Errorf("cannot execute OR assertions: %w", err)
		case tpm2.IsTPMParameterError(err, tpm2.ErrorValue, tpm2.CommandPolicyOR, 1):
			// The dynamic authorization policy data is invalid.
			return dynamicPolicyDataError{errors.New("cannot complete OR assertions: invalid data")}
		}
		return dynamicPolicyDataError{xerrors.Errorf("cannot complete OR assertions: %w", err)}
	}

	pcrPolicyCounterHandle := staticInput.pcrPolicyCounterHandle
	if (pcrPolicyCounterHandle != tpm2.HandleNull || version == 0) && pcrPolicyCounterHandle.Type() != tpm2.HandleTypeNVIndex {
		return staticPolicyDataError{errors.New("invalid handle for PCR policy counter")}
	}

	var policyCounter tpm2.ResourceContext
	if pcrPolicyCounterHandle != tpm2.HandleNull {
		var err error
		policyCounter, err = tpm.CreateResourceContextFromTPM(pcrPolicyCounterHandle)
		switch {
		case tpm2.IsResourceUnavailableError(err, pcrPolicyCounterHandle):
			// If there is no NV index at the expected handle then the key file is invalid and must be recreated.
			return staticPolicyDataError{errors.New("no PCR policy counter found")}
		case err != nil:
			return xerrors.Errorf("cannot obtain context for PCR policy counter: %w", err)
		}

		var revocationCheckSession tpm2.SessionContext
		if version == 0 {
			policyCounterPub, _, err := tpm.NVReadPublic(policyCounter)
			if err != nil {
				return xerrors.Errorf("cannot read public area for PCR policy counter: %w", err)
			}
			if !policyCounterPub.NameAlg.Available() {
				//If the NV index has an unsupported name algorithm, then this key file is invalid and must be recreated.
				return staticPolicyDataError{errors.New("PCR policy counter has an unsupported name algorithm")}
			}

			revocationCheckSession, err = tpm.StartAuthSession(nil, nil, tpm2.SessionTypePolicy, nil, policyCounterPub.NameAlg)
			if err != nil {
				return xerrors.Errorf("cannot create session for PCR policy revocation check: %w", err)
			}
			defer tpm.FlushContext(revocationCheckSession)

			// See the comment for computeV0PinNVIndexPostInitAuthPolicies for a description of the authorization policy
			// for the v0 NV index. Because the v0 NV index was also used for the PIN, it needed an authorization policy to
			// permit using the counter value in an assertion without knowing the authorization value of the index.
			if err := tpm.PolicyCommandCode(revocationCheckSession, tpm2.CommandPolicyNV); err != nil {
				return xerrors.Errorf("cannot execute assertion for PCR policy revocation check: %w", err)
			}
			if err := tpm.PolicyOR(revocationCheckSession, staticInput.v0PinIndexAuthPolicies); err != nil {
				if tpm2.IsTPMParameterError(err, tpm2.ErrorValue, tpm2.CommandPolicyOR, 1) {
					// staticInput.v0PinIndexAuthPolicies is invalid.
					return staticPolicyDataError{errors.New("authorization policy metadata for PCR policy counter is invalid")}
				}
				return xerrors.Errorf("cannot execute assertion for PCR policy revocation check: %w", err)
			}
		}

		operandB := make([]byte, 8)
		binary.BigEndian.PutUint64(operandB, dynamicInput.policyCount)
		if err := tpm.PolicyNV(policyCounter, policyCounter, policySession, operandB, 0, tpm2.OpUnsignedLE, revocationCheckSession); err != nil {
			switch {
			case tpm2.IsTPMError(err, tpm2.ErrorPolicy, tpm2.CommandPolicyNV):
				// The PCR policy has been revoked.
				return dynamicPolicyDataError{errors.New("the PCR policy has been revoked")}
			case tpm2.IsTPMSessionError(err, tpm2.ErrorPolicyFail, tpm2.CommandPolicyNV, 1):
				// Either staticInput.v0PinIndexAuthPolicies is invalid or the NV index isn't what's expected, so the key file is invalid.
				return staticPolicyDataError{errors.New("invalid PCR policy counter or associated authorization policy metadata")}
			}
			return xerrors.Errorf("PCR policy revocation check failed: %w", err)
		}
	}

	authPublicKey := staticInput.authPublicKey
	if !authPublicKey.NameAlg.Available() {
		return staticPolicyDataError{errors.New("public area of dynamic authorization policy signing key has an unsupported name algorithm")}
	}
	authorizeKey, err := tpm.LoadExternal(nil, authPublicKey, tpm2.HandleOwner)
	if err != nil {
		if tpm2.IsTPMParameterError(err, tpm2.AnyErrorCode, tpm2.CommandLoadExternal, 2) {
			// staticInput.AuthPublicKey is invalid
			return staticPolicyDataError{errors.New("public area of dynamic authorization policy signing key is invalid")}
		}
		return xerrors.Errorf("cannot load public area for dynamic authorization policy signing key: %w", err)
	}
	defer tpm.FlushContext(authorizeKey)

	var pcrPolicyRef tpm2.Nonce
	if version > 0 {
		// The authorized PCR policy signature contains a reference for > v0 metadata, which limits the scope of it for authorizing
		// PCR policy. In future, the key that authorizes this policy may be used to authorize other policy digests for the purposes of,
		// eg, recovery with a signed assertion.
		pcrPolicyRef = computePcrPolicyRefFromCounterContext(policyCounter)
	}

	h := authPublicKey.NameAlg.NewHash()
	h.Write(dynamicInput.authorizedPolicy)
	h.Write(pcrPolicyRef)

	authorizeTicket, err := tpm.VerifySignature(authorizeKey, h.Sum(nil), dynamicInput.authorizedPolicySignature)
	if err != nil {
		if tpm2.IsTPMParameterError(err, tpm2.AnyErrorCode, tpm2.CommandVerifySignature, 2) {
			// dynamicInput.AuthorizedPolicySignature or the computed policy ref is invalid.
			// XXX: It's not possible to determine whether this is broken dynamic or static metadata -
			//  we should just do away with the distinction here tbh
			return dynamicPolicyDataError{errors.New("cannot verify PCR policy signature")}
		}
		return xerrors.Errorf("cannot verify PCR policy signature: %w", err)
	}

	if err := tpm.PolicyAuthorize(policySession, dynamicInput.authorizedPolicy, pcrPolicyRef, authorizeKey.Name(), authorizeTicket); err != nil {
		if tpm2.IsTPMParameterError(err, tpm2.ErrorValue, tpm2.CommandPolicyAuthorize, 1) {
			// dynamicInput.AuthorizedPolicy is invalid.
			return dynamicPolicyDataError{errors.New("the PCR policy is invalid")}
		}
		return xerrors.Errorf("PCR policy check failed: %w", err)
	}

	if version == 0 {
		// For metadata version 0, PIN support was implemented by asserting knowlege of the authorization value
		// for the PCR policy counter, although this support was never used and has been removed.
		if _, _, err := tpm.PolicySecret(policyCounter, policySession, nil, nil, 0, hmacSession); err != nil {
			return xerrors.Errorf("cannot execute PolicySecret assertion: %w", err)
		}
	} else {
		// For metadata versions > 0, PIN support was implemented by requiring knowlege of the authorization value for
		// the sealed key object when this policy session is used to unseal it, although this support was never
		// used and has been removed.
		// XXX: This mechanism will be re-used as part of the passphrase integration in the future, although the
		//  authorization value will be a passphrase derived key.
		if err := tpm.PolicyAuthValue(policySession); err != nil {
			return xerrors.Errorf("cannot execute PolicyAuthValue assertion: %w", err)
		}
	}

	if version == 0 {
		// Execute required TPM2_PolicyNV assertion that was used for legacy locking with v0 files -
		// this is only here because the existing policy for v0 files requires it. It is not expected that
		// this will fail unless the NV index has been removed or altered, at which point the key is
		// non-recoverable anyway.
		index, err := tpm.CreateResourceContextFromTPM(lockNVHandle)
		if err != nil {
			return xerrors.Errorf("cannot obtain context for lock NV index: %w", err)
		}
		if err := tpm.PolicyNV(index, index, policySession, nil, 0, tpm2.OpEq, nil); err != nil {
			return xerrors.Errorf("policy lock check failed: %w", err)
		}
	}

	return nil
}

// BlockPCRProtectionPolicies inserts a fence in to the specific PCRs for all active PCR banks, in order to
// make PCR policies that depend on the specified PCRs and are satisfiable by the current PCR values invalid
// until the next TPM restart (equivalent to eg, system resume from suspend-to-disk) or TPM reset
// (equivalent to booting after a system reset).
//
// This acts as a barrier between the environment in which a sealed key should be permitted to be unsealed
// (eg, the initramfs), and the environment in which a sealed key should not be permitted to be unsealed
// (eg, the OS runtime).
func BlockPCRProtectionPolicies(tpm *Connection, pcrs []int) error {
	session := tpm.HmacSession()

	// The fence is a hash of uint32(0), which is the same as EV_SEPARATOR (which can be uint32(0) or uint32(-1))
	fence := make([]byte, 4)

	// Insert PCR fence
	for _, pcr := range pcrs {
		seq, err := tpm.HashSequenceStart(nil, tpm2.HashAlgorithmNull)
		if err != nil {
			return xerrors.Errorf("cannot being hash sequence: %w", err)
		}
		if _, err := tpm.EventSequenceExecute(tpm.PCRHandleContext(pcr), seq, fence, session, nil); err != nil {
			return xerrors.Errorf("cannot execute hash sequence: %w", err)
		}
	}

	return nil
}
