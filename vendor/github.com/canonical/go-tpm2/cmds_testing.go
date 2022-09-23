// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

// Section 9 - Start-up

func (t *TPMContext) SelfTest(fullTest bool, sessions ...SessionContext) error {
	return t.RunCommand(CommandSelfTest, sessions, Delimiter, fullTest)
}

func (t *TPMContext) IncrementalSelfTest(toTest AlgorithmList, sessions ...SessionContext) (AlgorithmList, error) {
	var toDoList AlgorithmList
	if err := t.RunCommand(CommandIncrementalSelfTest, sessions,
		Delimiter,
		toTest, Delimiter,
		Delimiter,
		&toDoList); err != nil {
		return nil, err
	}
	return toDoList, nil
}

func (t *TPMContext) GetTestResult(sessions ...SessionContext) (outData MaxBuffer, testResult ResponseCode, err error) {
	if err := t.RunCommand(CommandGetTestResult, sessions, Delimiter, Delimiter, Delimiter, &outData, &testResult); err != nil {
		return nil, 0, err
	}
	return outData, testResult, nil
}
