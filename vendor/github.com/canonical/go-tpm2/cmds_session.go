// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

// Section 11 - Session Commands

import (
	"fmt"

	"github.com/canonical/go-tpm2/internal"
)

// StartAuthSession executes the TPM2_StartAuthSession command to start an authorization session. On successful completion, it will
// return a SessionContext that corresponds to the new session.
//
// The type of session is defined by the sessionType parameter. If sessionType is SessionTypeHMAC or SessionTypePolicy, then the
// created session may be used for authorization. If sessionType is SessionTypeTrial, then the created session can only be used for
// computing an authorization policy digest.
//
// The authHash parameter defines the algorithm used for computing command and response parameter digests, command and response
// HMACs, and derivation of the session key and symmetric keys for parameter encryption where used. The size of the digest algorithm
// is used to determine the nonce size used for the session.
//
// If tpmKey is provided, it must correspond to an asymmetric decrypt key in the TPM. In this case, a random salt value will
// contribute to the session key derivation, and the salt will be encrypted using the method specified by tpmKey before being sent to
// the TPM. If tpmKey is provided but does not correspond to an asymmetric key, a *TPMHandleError error with an error code of ErrorKey
// will be returned for handle index 1. If tpmKey is provided but corresponds to an object with only its public part loaded, a
// *TPMHandleError error with an error code of ErrorHandle will be returned for handle index 1. If tpmKey is provided but does not
// correspond to a decrypt key, a *TPMHandleError error with an error code of ErrorAttributes will be returned for handle index 1.
//
// If tpmkey is provided but decryption of the salt fails on the TPM, a *TPMParameterError error with an error code of ErrorValue or
// ErrorKey may be returned for parameter index 2.
//
// If bind is specified, then the auhorization value for the corresponding resource must be known, by calling
// ResourceContext.SetAuthValue on bind before calling this function - the authorization value will contribute to the session key
// derivation. The created session will be bound to the resource associated with bind, unless the authorization value of that resource
// is subsequently changed. If bind corresponds to a transient object and only the public part of the object is loaded, or if bind
// corresponds to a NV index with a type of NVTypePinPass or NVTypePinFail, a *TPMHandleError error with an error code of ErrorHandle
// will be returned for handle index 2.
//
// If a session key is computed, this will be used (along with the authorization value of resources that the session is being used
// for authorization of if the session is not bound to them) to derive a HMAC key for generating command and response HMACs. If both
// tpmKey and bind are nil, no session key is created.
//
// If symmetric is provided, it defines the symmetric algorithm to use if the session is subsequently used for session based command
// or response parameter encryption. Session based parameter encryption allows the first command and/or response parameter for a
// command to be encrypted between the TPM and host CPU for supported parameter types (go types that correspond to TPM2B prefixed
// types). If symmetric is provided and corresponds to a symmetric block cipher (ie, the Algorithm field is not SymAlgorithmXOR) then
// the value of symmetric.Mode.Sym() must be SymModeCFB, else a *TPMParameterError error with an error code of ErrorMode is returned
// for parameter index 4.
//
// If a SessionContext instance with the AttrCommandEncrypt attribute set is provided in the variable length sessions parameter, then
// the initial caller nonce will be encrypted as this is the first command parameter, despite not being exposed via this API. If a
// SessionContext instance with the AttrResponseEncrypt attribute set is provided, then the initial TPM nonce will be encrypted in the
// response.
//
// If sessionType is SessionTypeHMAC and the session is subsequently used for authorization of a resource to which the session is not
// bound, the authorization value of that resource must be known as it is used to derive the key for computing command and response
// HMACs.
//
// If no more sessions can be created without first context loading the oldest saved session, then a *TPMWarning error with a warning
// code of WarningContextGap will be returned. If there are no more slots available for loaded sessions, a *TPMWarning error with a
// warning code of WarningSessionMemory will be returned. If there are no more session handles available, a *TPMwarning error with
// a warning code of WarningSessionHandles will be returned.
func (t *TPMContext) StartAuthSession(tpmKey, bind ResourceContext, sessionType SessionType, symmetric *SymDef, authHash HashAlgorithmId, sessions ...SessionContext) (sessionContext SessionContext, err error) {
	if symmetric == nil {
		symmetric = &SymDef{Algorithm: SymAlgorithmNull}
	}
	if !authHash.Available() {
		return nil, makeInvalidArgError("authHash", fmt.Sprintf("unsupported digest algorithm or algorithm not linked in to binary (%v)", authHash))
	}
	digestSize := authHash.Size()

	var salt []byte
	var encryptedSalt EncryptedSecret
	tpmKeyHandle := HandleNull
	if tpmKey != nil {
		object, isObject := tpmKey.(*objectContext)
		if !isObject {
			return nil, makeInvalidArgError("tpmKey", "resource context is not an object")
		}

		tpmKeyHandle = tpmKey.Handle()

		var err error
		encryptedSalt, salt, err = CryptSecretEncrypt(object.GetPublic(), []byte(SecretKey))
		if err != nil {
			return nil, fmt.Errorf("cannot compute encrypted salt: %v", err)
		}
	}

	var authValue []byte
	bindHandle := HandleNull
	if bind != nil {
		bindHandle = bind.Handle()
		authValue = bind.(resourceContextPrivate).GetAuthValue()
	}

	var isBound bool = false
	var boundEntity Name
	if bindHandle != HandleNull && sessionType == SessionTypeHMAC {
		boundEntity = computeBindName(bind.Name(), authValue)
		isBound = true
	}

	nonceCaller := make([]byte, digestSize)
	if err := cryptComputeNonce(nonceCaller); err != nil {
		return nil, fmt.Errorf("cannot compute initial nonceCaller: %v", err)
	}

	var sessionHandle Handle
	var nonceTPM Nonce

	if err := t.RunCommand(CommandStartAuthSession, sessions,
		tpmKey, bind, Delimiter,
		Nonce(nonceCaller), encryptedSalt, sessionType, symmetric, authHash, Delimiter,
		&sessionHandle, Delimiter,
		&nonceTPM); err != nil {
		return nil, err
	}

	switch sessionHandle.Type() {
	case HandleTypeHMACSession, HandleTypePolicySession:
	default:
		return nil, &InvalidResponseError{CommandStartAuthSession,
			fmt.Sprintf("handle 0x%08x returned from TPM is the wrong type", sessionHandle)}
	}

	data := &sessionContextData{
		HashAlg:        authHash,
		SessionType:    sessionType,
		PolicyHMACType: policyHMACTypeNoAuth,
		IsBound:        isBound,
		BoundEntity:    boundEntity,
		NonceCaller:    nonceCaller,
		NonceTPM:       nonceTPM,
		Symmetric:      symmetric}

	if tpmKeyHandle != HandleNull || bindHandle != HandleNull {
		key := make([]byte, len(authValue)+len(salt))
		copy(key, authValue)
		copy(key[len(authValue):], salt)

		data.SessionKey = internal.KDFa(authHash.GetHash(), key, []byte(SessionKey), []byte(nonceTPM), nonceCaller, digestSize*8)
	}

	return makeSessionContext(sessionHandle, data), nil
}

// PolicyRestart executes the TPM2_PolicyRestart command on the policy session associated with sessionContext, to reset the policy
// authorization session to its initial state.
func (t *TPMContext) PolicyRestart(sessionContext SessionContext, sessions ...SessionContext) error {
	return t.RunCommand(CommandPolicyRestart, sessions, sessionContext)
}
