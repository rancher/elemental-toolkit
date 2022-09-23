// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

// Section 22 - Integrity Collection (PCR)

// PCRExtend executes the TPM2_PCR_Extend command to extend the PCR associated with the pcrContext parameter with the tagged digests
// provided via the digests argument. If will iterate over the digests and extend the PCR with each one for the PCR bank associated
// with the algorithm for each digest.
//
// If pcrContext is nil, this function will do nothing. The command requires authorization with the user auth role for pcrContext,
// with session based authorization provided via pcrContextAuthSession.
//
// If the PCR associated with pcrContext can not be extended from the current locality, a *TPMError error with an error code of
// ErrorLocality will be returned.
func (t *TPMContext) PCRExtend(pcrContext ResourceContext, digests TaggedHashList, pcrContextAuthSession SessionContext, sessions ...SessionContext) error {
	return t.RunCommand(CommandPCRExtend, sessions,
		ResourceContextWithSession{Context: pcrContext, Session: pcrContextAuthSession}, Delimiter,
		digests)
}

// PCREvent executes the TPM2_PCR_Event command to extend the PCR associated with the pcrContext parameter with a digest of the
// provided eventData, hashed with the algorithm for each supported PCR bank.
//
// If pcrContext is nil, this function will do nothing. The command requires authorization with the user auth role for pcrContext,
// with session based authorization provided via pcrContextAuthSession.
//
// If the PCR associated with pcrContext can not be extended from the current locality, a *TPMError error with an error code of
// ErrorLocality will be returned.
//
// On success, this function will return a list of tagged digests that the PCR associated with pcrContext was extended with.
func (t *TPMContext) PCREvent(pcrContext ResourceContext, eventData Event, pcrContextAuthSession SessionContext, sessions ...SessionContext) (digests TaggedHashList, err error) {
	if err := t.RunCommand(CommandPCREvent, sessions,
		ResourceContextWithSession{Context: pcrContext, Session: pcrContextAuthSession}, Delimiter,
		eventData, Delimiter,
		Delimiter,
		&digests); err != nil {
		return nil, err
	}
	return digests, nil
}

// PCRRead executes the TPM2_PCR_Read command to return the values of the PCRs defined in the pcrSelectionIn parameter. The
// underlying command may not be able to read all of the specified PCRs in a single transaction, so this function will
// re-execute the TPM2_PCR_Read command until all requested values have been read. As a consequence, any SessionContext instances
// provided should have the AttrContinueSession attribute defined.
//
// On success, the current value of pcrUpdateCounter is returned, as well as the requested PCR values.
func (t *TPMContext) PCRRead(pcrSelectionIn PCRSelectionList, sessions ...SessionContext) (pcrUpdateCounter uint32, pcrValues PCRValues, err error) {
	var remaining PCRSelectionList
	for _, s := range pcrSelectionIn {
		c := PCRSelection{Hash: s.Hash, Select: make([]int, len(s.Select))}
		copy(c.Select, s.Select)
		remaining = append(remaining, c)
	}

	pcrValues = make(PCRValues)

	for i := 0; ; i++ {
		var updateCounter uint32
		var pcrSelectionOut PCRSelectionList
		var values DigestList

		if err := t.RunCommand(CommandPCRRead, sessions,
			Delimiter,
			remaining, Delimiter,
			Delimiter,
			&updateCounter, &pcrSelectionOut, &values); err != nil {
			return 0, nil, err
		}

		if i == 0 {
			pcrUpdateCounter = updateCounter
		} else if updateCounter != pcrUpdateCounter {
			return 0, nil, &InvalidResponseError{CommandPCRRead, "PCR update counter changed between commands"}
		} else if len(values) == 0 && pcrSelectionOut.IsEmpty() {
			return 0, nil, makeInvalidArgError("pcrSelectionIn", "unimplemented PCRs specified")
		}

		if n, err := pcrValues.SetValuesFromListAndSelection(pcrSelectionOut, values); err != nil {
			return 0, nil, &InvalidResponseError{CommandPCRRead, err.Error()}
		} else if n != len(values) {
			return 0, nil, &InvalidResponseError{CommandPCRRead, "too many digests"}
		}

		remaining = remaining.Remove(pcrSelectionOut)
		if remaining.IsEmpty() {
			break
		}
	}

	return pcrUpdateCounter, pcrValues, nil
}

// PCRReset executes the TPM2_PCR_Reset command to reset the PCR associated with pcrContext in all banks. This command requires
// authorization with the user auth role for pcrContext, with session based authorization provided via pcrContextAuthSession.
//
// If the PCR associated with pcrContext can not be reset from the current locality, a *TPMError error with an error code of
// ErrorLocality will be returned.
func (t *TPMContext) PCRReset(pcrContext ResourceContext, pcrContextAuthSession SessionContext, sessions ...SessionContext) error {
	return t.RunCommand(CommandPCRReset, sessions, ResourceContextWithSession{Context: pcrContext, Session: pcrContextAuthSession})
}
