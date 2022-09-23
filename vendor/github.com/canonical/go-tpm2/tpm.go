// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/canonical/go-tpm2/mu"

	"golang.org/x/xerrors"
)

func makeInvalidArgError(name, msg string) error {
	return fmt.Errorf("invalid %s argument: %s", name, msg)
}

func isSessionAllowed(commandCode CommandCode) bool {
	switch commandCode {
	case CommandStartup:
		return false
	case CommandContextLoad:
		return false
	case CommandContextSave:
		return false
	case CommandFlushContext:
		return false
	default:
		return true
	}
}

type cmdContext struct {
	commandCode      CommandCode
	sessionParams    *sessionParams
	responseCode     ResponseCode
	responseAuthArea []AuthResponse
	rpBytes          []byte
}

type delimiterSentinel struct{}

// Delimiter is a sentinel value used to delimit command handle, command parameter, response handle pointer and response
// parameter pointer blocks in the variable length params argument in TPMContext.RunCommand.
var Delimiter delimiterSentinel

// ResourceContextWithAuth associates a ResourceContext with a session for authorization, and is provided to TPMContext.RunCommand in
// the command handle area for any handles that require an authorization.
type ResourceContextWithSession struct {
	Context ResourceContext
	Session SessionContext
}

// TODO: Implement commands from the following sections of part 3 of the TPM library spec:
// Section 14 - Asymmetric Primitives
// Section 15 - Symmetric Primitives
// Section 17 - Hash/HMAC/Event Sequences
// Section 19 - Ephemeral EC Keys
// Section 26 - Miscellaneous Management Functions
// Section 27 - Field Upgrade

// TPMContext is the main entry point by which commands are executed on a TPM device using this package. It communicates with the
// underlying device via a transmission interface, which is an implementation of io.ReadWriteCloser provided to NewTPMContext.
//
// Methods that execute commands on the TPM will return errors where the TPM responds with them. These are in the form of *TPMError,
// *TPMWarning, *TPMHandleError, *TPMSessionError, *TPMParameterError and *TPMVendorError types.
//
// Some methods also accept a variable number of optional SessionContext arguments - these are for sessions that don't provide
// authorization for a corresponding TPM resource. These sessions may be used for the purposes of session based parameter encryption
// or command auditing.
type TPMContext struct {
	tcti                  TCTI
	permanentResources    map[Handle]*permanentContext
	maxSubmissions        uint
	propertiesInitialized bool
	maxBufferSize         int
	maxDigestSize         int
	maxNVBufferSize       int
	exclusiveSession      *sessionContext
}

// Close calls Close on the transmission interface.
func (t *TPMContext) Close() error {
	if err := t.tcti.Close(); err != nil {
		return &TctiError{"close", err}
	}

	return nil
}

// RunCommandBytes is a low-level interface for executing a command. The caller is responsible for supplying a properly
// serialized command packet, which can be created with MarshalCommandPacket.
//
// If successful, this function will return the response packet. An error will only be returned if the transmission
// interface returns an error.
func (t *TPMContext) RunCommandBytes(packet CommandPacket) (ResponsePacket, error) {
	if _, err := t.tcti.Write(packet); err != nil {
		return nil, &TctiError{"write", err}
	}

	resp, err := ioutil.ReadAll(t.tcti)
	if err != nil {
		return nil, &TctiError{"read", err}
	}

	return ResponsePacket(resp), nil
}

func (t *TPMContext) runCommandWithoutProcessingAuthResponse(commandCode CommandCode, sessionParams *sessionParams, inHandles []HandleContext, params []interface{}, outHandle *Handle) (*cmdContext, error) {
	handles := make(HandleList, 0, len(inHandles))
	handleNames := make([]Name, 0, len(inHandles))

	for _, h := range inHandles {
		handles = append(handles, h.Handle())
		handleNames = append(handleNames, h.Name())
	}

	if sessionParams.hasDecryptSession() && (len(params) == 0 || !isParamEncryptable(params[0])) {
		return nil, fmt.Errorf("command %s does not support command parameter encryption", commandCode)
	}

	cpBytes, err := mu.MarshalToBytes(params...)
	if err != nil {
		return nil, xerrors.Errorf("cannot marshal parameters for command %s: %w", commandCode, err)
	}

	cAuthArea, err := sessionParams.buildCommandAuthArea(commandCode, handleNames, cpBytes)
	if err != nil {
		return nil, xerrors.Errorf("cannot build auth area for command %s: %w", commandCode, err)
	}

	cmd := MarshalCommandPacket(commandCode, handles, cAuthArea, cpBytes)

	var responseCode ResponseCode
	var rpBytes []byte
	var rAuthArea []AuthResponse

	for tries := uint(1); ; tries++ {
		var err error
		resp, err := t.RunCommandBytes(cmd)
		if err != nil {
			return nil, err
		}

		responseCode, rpBytes, rAuthArea, err = resp.Unmarshal(outHandle)
		if err != nil {
			return nil, &InvalidResponseError{commandCode, fmt.Sprintf("cannot unmarshal response packet: %v", err)}
		}

		err = DecodeResponseCode(commandCode, responseCode)
		if _, invalidRc := err.(InvalidResponseCodeError); invalidRc {
			return nil, &InvalidResponseError{commandCode, err.Error()}
		}
		if err == nil {
			if len(rAuthArea) != len(sessionParams.sessions) {
				return nil, &InvalidResponseError{commandCode, fmt.Sprintf("unexpected number of auth responses (got %d, expected %d)",
					len(rAuthArea), len(sessionParams.sessions))}
			}

			break
		}

		if tries >= t.maxSubmissions {
			return nil, err
		}
		if !(IsTPMWarning(err, WarningYielded, commandCode) || IsTPMWarning(err, WarningTesting, commandCode) || IsTPMWarning(err, WarningRetry, commandCode)) {
			return nil, err
		}
	}

	return &cmdContext{
		commandCode:      commandCode,
		sessionParams:    sessionParams,
		responseCode:     responseCode,
		responseAuthArea: rAuthArea,
		rpBytes:          rpBytes}, nil
}

func (t *TPMContext) processAuthResponse(cmd *cmdContext, params []interface{}) error {
	if len(cmd.responseAuthArea) > 0 {
		if err := cmd.sessionParams.processResponseAuthArea(cmd.responseAuthArea, cmd.responseCode, cmd.rpBytes); err != nil {
			return &InvalidResponseError{cmd.commandCode, fmt.Sprintf("cannot process response auth area: %v", err)}
		}
	}

	if isSessionAllowed(cmd.commandCode) {
		if t.exclusiveSession != nil {
			t.exclusiveSession.Data().IsExclusive = false
		}
		var exclusive *sessionContext
		for _, s := range cmd.sessionParams.sessions {
			if s.session == nil {
				continue
			}
			if s.session.Data().IsExclusive {
				exclusive = s.session
				break
			}
		}
		t.exclusiveSession = exclusive
		if t.exclusiveSession != nil {
			t.exclusiveSession.Data().IsExclusive = true
		}
	}

	rpBuf := bytes.NewReader(cmd.rpBytes)

	if _, err := mu.UnmarshalFromReader(rpBuf, params...); err != nil {
		return &InvalidResponseError{cmd.commandCode, fmt.Sprintf("cannot unmarshal response parameters: %v", err)}
	}

	if rpBuf.Len() > 0 {
		return &InvalidResponseError{cmd.commandCode, fmt.Sprintf("response parameter area contains %d trailing bytes", rpBuf.Len())}
	}

	return nil
}

// RunCommandWithResponseCallback is a high-level generic interface for executing the command specified by commandCode. It differs
// from RunCommand with the addition of an optional callback which is executed after receiving a response from the TPM, but before
// the response is decoded and the session state is updated. This is useful for commands that change the authorization value of a
// supplied entity, where the response HMAC may be generated based on the new authorization value. It takes care of marshalling
// command handles and command parameters, as well as constructing and marshalling the authorization area and choosing the correct
// StructTag value. It takes care of unmarshalling response handles and response parameters, as well as unmarshalling the response
// authorization area and performing checks on the authorization response.
//
// The variable length params argument provides a mechanism for the caller to provide command handles, command parameters, response
// handle pointers and response parameter pointers (in that order), with each group of arguments being separated by the Delimiter
// sentinel value.
//
// Command handles are provided as HandleContext types if they do not require an authorization. For command handles that require an
// authorization, they are provided using the ResourceContextWithSession type. This links the ResourceContext to an optional
// authorization session. If the authorization value of the TPM entity is required as part of the authorization, this will be obtained
// from the supplied ResourceContext. A nil HandleContext will automatically be converted to a handle with the value of HandleNull.
//
// Command parameters are provided as the go equivalent types for the types defined in the TPM Library Specification.
//
// Response handles are provided as pointers to Handle values.
//
// Response parameters are provided as pointers to values of the go equivalent types for the types defined in the TPM Library
// Specification.
//
// If the TPM responds with a warning that indicates the command could not be started and should be retried, this function will
// resubmit the command a finite number of times before returning an error. The maximum number of retries can be set via
// TPMContext.SetMaxSubmissions.
//
// The caller can provide additional sessions that aren't associated with a TPM entity (and therefore not used for authorization) via
// the sessions parameter, for the purposes of command auditing or session based parameter encryption.
//
// In addition to returning an error if any marshalling or unmarshalling fails, or if the transmission backend returns an error,
// this function will also return an error if the TPM responds with any ResponseCode other than Success.
func (t *TPMContext) RunCommandWithResponseCallback(commandCode CommandCode, sessions []SessionContext, responseCb func(), params ...interface{}) error {
	var commandHandles []HandleContext
	var commandParams []interface{}
	var responseHandle *Handle
	var responseParams []interface{}
	var sessionParams sessionParams

	sentinels := 0
	for _, param := range params {
		if param == Delimiter {
			sentinels++
			continue
		}

		switch sentinels {
		case 0:
			var handle HandleContext
			switch p := param.(type) {
			case ResourceContextWithSession:
				handle = p.Context
				if err := sessionParams.validateAndAppendAuth(p); err != nil {
					return fmt.Errorf("cannot process ResourceContextWithSession for command %s at index %d: %v", commandCode, len(commandHandles), err)
				}
			case HandleContext:
				handle = p
			case nil:
			default:
				return fmt.Errorf("cannot process command handle argument for command %s at index %d: invalid type (%s)", commandCode, len(commandHandles), reflect.TypeOf(param))
			}
			if handle == nil {
				handle = makePermanentContext(HandleNull)
			}
			commandHandles = append(commandHandles, handle)
		case 1:
			commandParams = append(commandParams, param)
		case 2:
			if responseHandle != nil {
				return errors.New("only one response handle argument can be supplied")
			}
			handle, isHandle := param.(*Handle)
			if !isHandle {
				return fmt.Errorf("cannot process response handle argument for command %s: invalid type (%s)", commandCode, reflect.TypeOf(param))
			}
			responseHandle = handle
		case 3:
			responseParams = append(responseParams, param)
		}
	}

	if err := sessionParams.validateAndAppendExtra(sessions); err != nil {
		return fmt.Errorf("cannot process non-auth SessionContext parameters for command %s: %v", commandCode, err)
	}

	ctx, err := t.runCommandWithoutProcessingAuthResponse(commandCode, &sessionParams, commandHandles, commandParams, responseHandle)
	if err != nil {
		return err
	}

	if responseCb != nil {
		responseCb()
	}

	return t.processAuthResponse(ctx, responseParams)
}

// RunCommand is the high-level generic interface for executing the command specified by commandCode. All of the methods on TPMContext
// exported by this package that execute commands on the TPM are essentially wrappers around this function. It takes care of
// marshalling command handles and command parameters, as well as constructing and marshalling the authorization area and choosing
// the correct StructTag value. It takes care of unmarshalling response handles and response parameters, as well as unmarshalling the
// response authorization area and performing checks on the authorization response.
//
// The variable length params argument provides a mechanism for the caller to provide command handles, command parameters, response
// handle pointers and response parameter pointers (in that order), with each group of arguments being separated by the Delimiter
// sentinel value.
//
// Command handles are provided as HandleContext types if they do not require an authorization. For command handles that require an
// authorization, they are provided using the ResourceContextWithSession type. This links the ResourceContext to an optional
// authorization session. If the authorization value of the TPM entity is required as part of the authorization, this will be obtained
// from the supplied ResourceContext. A nil HandleContext will automatically be converted to a handle with the value of HandleNull.
//
// Command parameters are provided as the go equivalent types for the types defined in the TPM Library Specification.
//
// Response handles are provided as pointers to Handle values.
//
// Response parameters are provided as pointers to values of the go equivalent types for the types defined in the TPM Library
// Specification.
//
// If the TPM responds with a warning that indicates the command could not be started and should be retried, this function will
// resubmit the command a finite number of times before returning an error. The maximum number of retries can be set via
// TPMContext.SetMaxSubmissions.
//
// The caller can provide additional sessions that aren't associated with a TPM entity (and therefore not used for authorization) via
// the sessions parameter, for the purposes of command auditing or session based parameter encryption.
//
// In addition to returning an error if any marshalling or unmarshalling fails, or if the transmission backend returns an error,
// this function will also return an error if the TPM responds with any ResponseCode other than Success.
func (t *TPMContext) RunCommand(commandCode CommandCode, sessions []SessionContext, params ...interface{}) error {
	return t.RunCommandWithResponseCallback(commandCode, sessions, nil, params...)
}

// SetMaxSubmissions sets the maximum number of times that RunCommand will attempt to submit a command before failing with an error.
// The default value is 5.
func (t *TPMContext) SetMaxSubmissions(max uint) {
	t.maxSubmissions = max
}

// InitProperties executes a TPM2_GetCapability command to initialize properties used internally by TPMContext. This is normally done
// automatically by functions that require these properties when they are used for the first time, but this function is provided so
// that the command can be audited, and so the exclusivity of an audit session can be preserved.
func (t *TPMContext) InitProperties(sessions ...SessionContext) error {
	props, err := t.GetCapabilityTPMProperties(PropertyFixed, CapabilityMaxProperties, sessions...)
	if err != nil {
		return err
	}

	for _, prop := range props {
		switch prop.Property {
		case PropertyInputBuffer:
			t.maxBufferSize = int(prop.Value)
		case PropertyMaxDigest:
			t.maxDigestSize = int(prop.Value)
		case PropertyNVBufferMax:
			t.maxNVBufferSize = int(prop.Value)
		}
	}

	if t.maxBufferSize == 0 {
		t.maxBufferSize = 1024
	}
	if t.maxDigestSize == 0 {
		return &InvalidResponseError{Command: CommandGetCapability, msg: "missing or invalid TPM_PT_MAX_DIGEST property"}
	}
	if t.maxNVBufferSize == 0 {
		return &InvalidResponseError{Command: CommandGetCapability, msg: "missing or invalid TPM_PT_NV_BUFFER_MAX property"}
	}
	t.propertiesInitialized = true
	return nil
}

func (t *TPMContext) initPropertiesIfNeeded() error {
	if t.propertiesInitialized {
		return nil
	}
	return t.InitProperties()
}

func newTpmContext(tcti TCTI) *TPMContext {
	r := new(TPMContext)
	r.tcti = tcti
	r.permanentResources = make(map[Handle]*permanentContext)
	r.maxSubmissions = 5

	return r
}

// NewTPMContext creates a new instance of TPMContext, which communicates with the
// TPM using the transmission interface provided via the tcti parameter. The
// transmission interface must not be nil - it is expected that the caller checks
// the error returned from the function that is used to create it.
func NewTPMContext(tcti TCTI) *TPMContext {
	if tcti == nil {
		panic("nil transmission interface")
	}

	t := new(TPMContext)
	t.tcti = tcti
	t.permanentResources = make(map[Handle]*permanentContext)
	t.maxSubmissions = 5

	return t
}
