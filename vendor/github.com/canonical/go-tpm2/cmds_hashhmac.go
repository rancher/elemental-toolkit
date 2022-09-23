// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

// Section 17 - Hash/HMAC/Event Sequences

// HMACStart executes the TPM2_HMAC_Start command to begin a HMAC sequence. The context argument corresponds to a loaded HMAC
// key. This command requires authorization with the user auth role for context, with session based authorization provided via
// contextAuthSession. The command creates a new HMAC sequence object on the TPM. The auth argument defines the authorization value
// for the newly created sequence object, which is required for subsequent use of it.
//
// If context does not correspond to an object with the type ObjectTypeKeyedHash, a *TPMHandleError error with an error code of
// ErrorType will be returned.
//
// If context corresponds to an object with the AttrRestricted attribute set, a *TPMHandleError error with an error code of
// ErrorAttributes will be returned.
//
// If context does not correspond to a signing key, a *TPMHandleError error with an error code of ErrorKey will be returned.
//
// The hashAlg argument specifies the HMAC algorithm. If the default scheme of the key associated with context is KeyedHashSchemeNull,
// then hashAlg must not be HashAlgorithmNull. If the default scheme of the key associated with context is not KeyedHashSchemeNull,
// then hashAlg must either be HashAlgorithmNull or must match the key's default scheme, else a *TPMParameterError error with an error
// code of ErrorValue will be returned for parameter index 2.
//
// On success, a ResourceContext corresponding to the newly created HMAC sequence object will be returned. It will not be necessary
// to call ResourceContext.SetAuthValue on it - this function sets the correct authorization value so that it can be used in
// subsequent commands that require knowledge of the authorization value.
func (t *TPMContext) HMACStart(context ResourceContext, auth Auth, hashAlg HashAlgorithmId, contextAuthSession SessionContext, sessions ...SessionContext) (sequenceContext ResourceContext, err error) {
	var sequenceHandle Handle

	if err := t.RunCommand(CommandHMACStart, sessions,
		ResourceContextWithSession{Context: context, Session: contextAuthSession}, Delimiter,
		auth, hashAlg, Delimiter,
		&sequenceHandle); err != nil {
		return nil, err
	}

	rc := makeObjectContext(sequenceHandle, nil, nil)
	rc.authValue = make([]byte, len(auth))
	copy(rc.authValue, auth)
	return rc, nil
}

// HashSequenceStart executes the TPM2_HashSequenceStart command to begin a hash or event sequence. The command creates a new
// sequence object on the TPM. The auth argument defines the authorization value for the newly created sequence object, which is
// required for subsequent use of it.
//
// If hashAlg is HashAlgorithmNull, this function will return a ResourceContext corresponding to a newly created event sequence
// object. If hashAlg is not HashAlgorithmNull, this function will return a ResourceContext corresponding to a newly created hash
// sequence object. It will not be necessary to call ResourceContext.SetAuthValue on it - this function sets the correct authorization
// value so that it can be used in subsequent commands that require knowledge of the authorization value.
func (t *TPMContext) HashSequenceStart(auth Auth, hashAlg HashAlgorithmId, sessions ...SessionContext) (sequenceContext ResourceContext, err error) {
	var sequenceHandle Handle

	if err := t.RunCommand(CommandHashSequenceStart, sessions,
		Delimiter,
		auth, hashAlg, Delimiter,
		&sequenceHandle); err != nil {
		return nil, err
	}

	rc := makeObjectContext(sequenceHandle, nil, nil)
	rc.authValue = make([]byte, len(auth))
	copy(rc.authValue, auth)
	return rc, nil
}

// SequenceUpdate executes the TPM2_SequenceUpdate command to add data to the HMAC, hash or event sequence associated with
// sequenceContext. This command requires authorization with the user auth role for sequenceContext, with session based authorization
// provided via sequenceContextAuthSession.
//
// If sequenceContext does not correspond to a sequence object, then a *TPMHandleError error with an error code of ErrorMode will be
// returned.
//
// If sequenceContext corresponds to a hash sequence and the hash sequence is intended to produce a digest that will be signed with
// a restricted signing key, the first block of data added to this sequence must be 4 bytes and not the value of TPMGeneratedValue.
func (t *TPMContext) SequenceUpdate(sequenceContext ResourceContext, buffer MaxBuffer, sequenceContextAuthSession SessionContext, sessions ...SessionContext) error {
	return t.RunCommand(CommandSequenceUpdate, sessions,
		ResourceContextWithSession{Context: sequenceContext, Session: sequenceContextAuthSession}, Delimiter,
		buffer)
}

// SequenceComplete executes the TPM2_SequenceComplete command to add the last part of the data the HMAC or hash sequence associated
// with sequenceContext, and returns the result. This command requires authorization with the user auth role for sequenceContext, with
// session based authorization provided via sequenceContextAuthSession.
//
// If sequenceContext does not correspond to a HMAC or hash sequence object, then a *TPMHandleError error with an error code of
// ErrorMode will be returned.
//
// If sequenceContext corresponds to a hash sequence and the hash sequence is intended to produce a digest that will be signed with
// a restricted signing key, the first block of data added to this sequence must be 4 bytes and not the value of TPMGeneratedValue.
// If the returned digest is safe to sign with a restricted signing key, then a ticket that can be passed to TPMContext.Sign will be
// returned. In this case, the hierarchy argument is used to specify the hierarchy for the ticket.
//
// On success, the sequence object associated with sequenceContext will be evicted, and sequenceContext will become invalid.
func (t *TPMContext) SequenceComplete(sequenceContext ResourceContext, buffer MaxBuffer, hierarchy Handle, sequenceContextAuthSession SessionContext, sessions ...SessionContext) (result Digest, validation *TkHashcheck, err error) {
	if err := t.RunCommand(CommandSequenceComplete, sessions,
		ResourceContextWithSession{Context: sequenceContext, Session: sequenceContextAuthSession}, Delimiter,
		buffer, hierarchy, Delimiter,
		Delimiter,
		&result, &validation); err != nil {
		return nil, nil, err
	}

	if validation.Hierarchy == HandleNull && len(validation.Digest) == 0 {
		validation = nil
	}

	sequenceContext.(handleContextPrivate).invalidate()
	return result, validation, nil
}

// EventSequenceComplete executes the TPM2_EventSequenceComplete command to add the last part of the data to the event sequence
// associated with sequenceContext, and return the result. This command requires authorization with the user auth role for
// sequenceContext, with session based authorization provided via sequenceContextAuthSession.
//
// If pcrContext is not nil, the result will be extended to the corresponding PCR in the same manner as TPMContext.PCRExtend.
// Authorization with the user auth role is required for pcrContext, with session based authorization provided via
// pcrContextAuthSession.
//
// If sequenceContext does not correspond to an event sequence object, then a *TPMHandleError error with an error code of ErrorMode
// will be returned for handle index 2.
//
// If pcrContext is not nil and the corresponding PCR can not be extended from the current locality, a *TPMError error with an
// error code of ErrorLocality will be returned.
//
// On success, the sequence object associated with sequenceContext will be evicted, and sequenceContext will become invalid.
func (t *TPMContext) EventSequenceComplete(pcrContext, sequenceContext ResourceContext, buffer MaxBuffer, pcrContextAuthSession, sequenceContextAuthSession SessionContext, sessions ...SessionContext) (results TaggedHashList, err error) {
	if err := t.RunCommand(CommandEventSequenceComplete, sessions,
		ResourceContextWithSession{Context: pcrContext, Session: pcrContextAuthSession}, ResourceContextWithSession{Context: sequenceContext, Session: sequenceContextAuthSession}, Delimiter,
		buffer, Delimiter,
		Delimiter,
		&results); err != nil {
		return nil, err
	}

	sequenceContext.(handleContextPrivate).invalidate()
	return results, nil
}

// SequenceExecute executes a hash or HMAC sequence to completion and returns the result by adding the provided data to the sequence
// with a number of TPM2_SequenceUpdate commands appropriate for the size of buffer, and executing a final TPM2_SequenceComplete
// command. This command requires authorization with the user auth role for sequenceContext, with session based authorization provided
// via sequenceContextAuthSession. As this function executes multiple commands, any SessionContext instances provided should have the
// AttrContinueSession attribute defined.
//
// If sequenceContext does not correspond to a hash or HMAC sequence object, then a *TPMHandleError error with an error code of
// ErrorMode will be returned.
//
// If sequenceContext corresponds to a hash sequence and the hash sequence is intended to produce a digest that will be signed with
// a restricted signing key, the first block of data added to this sequence must be 4 bytes and not the value of TPMGeneratedValue.
// If the returned digest is safe to sign with a restricted signing key, then a ticket that can be passed to TPMContext.Sign will be
// returned. In this case, the hierarchy argument is used to specify the hierarchy for the ticket.
//
// On success, the sequence object associated with sequenceContext will be evicted, and sequenceContext will become invalid.
func (t *TPMContext) SequenceExecute(sequenceContext ResourceContext, buffer []byte, hierarchy Handle, sequenceContextAuthSession SessionContext, sessions ...SessionContext) (result Digest, validation *TkHashcheck, err error) {
	if err := t.initPropertiesIfNeeded(); err != nil {
		return nil, nil, err
	}

	total := 0
	for len(buffer)-total > t.maxBufferSize {
		b := buffer[total:]
		b = b[:t.maxBufferSize]
		if err := t.SequenceUpdate(sequenceContext, b, sequenceContextAuthSession, sessions...); err != nil {
			return nil, nil, err
		}

		total += len(b)
	}

	return t.SequenceComplete(sequenceContext, buffer[total:], hierarchy, sequenceContextAuthSession, sessions...)
}

// EventSequenceExecute executes an event sequence to completion and returns the result by adding the provided data to the sequence
// with a number of TPM2_SequenceUpdate commands appropriate for the size of buffer, and executing a final TPM2_EventSequenceComplete
// command. This command requires authorization with the user auth role for sequenceContext, with session based authorization provided
// via sequenceContextAuthSession.
//
// If pcrContext is not nil, the result will be extended to the corresponding PCR in the same manner as TPMContext.PCRExtend.
// Authorization with the user auth role is required for pcrContext, with session based authorization provided via
// pcrContextAuthSession.
//
// As this function executes multiple commands, any SessionContext instances provided should have the AttrContinueSession attribute
// defined.
//
// If sequenceContext does not correspond to an event sequence object, then a *TPMHandleError error with an error code of ErrorMode
// will be returned for handle index 1 if the command is CommandSequenceUpdate, or handle index 2 if the command is
// CommandEventSequenceComplete.
//
// If pcrContext is not nil and the corresponding PCR can not be extended from the current locality, a *TPMError error with an
// error code of ErrorLocality will be returned.
//
// On success, the sequence object associated with sequenceContext will be evicted, and sequenceContext will become invalid.
func (t *TPMContext) EventSequenceExecute(pcrContext, sequenceContext ResourceContext, buffer []byte, pcrContextAuthSession, sequenceContextAuthSession SessionContext, sessions ...SessionContext) (results TaggedHashList, err error) {
	if err := t.initPropertiesIfNeeded(); err != nil {
		return nil, err
	}

	total := 0
	for len(buffer)-total > t.maxBufferSize {
		b := buffer[total:]
		b = b[:t.maxBufferSize]
		if err := t.SequenceUpdate(sequenceContext, b, sequenceContextAuthSession, sessions...); err != nil {
			return nil, err
		}

		total += len(b)
	}

	return t.EventSequenceComplete(pcrContext, sequenceContext, buffer[total:], pcrContextAuthSession, sequenceContextAuthSession, sessions...)
}
