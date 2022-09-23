// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

import (
	"bytes"
	"crypto/hmac"
	"errors"
	"fmt"
	"hash"
)

type policyHMACType uint8

const (
	policyHMACTypeNoAuth policyHMACType = iota
	policyHMACTypeAuth
	policyHMACTypePassword

	policyHMACTypeMax = policyHMACTypePassword
)

type sessionParam struct {
	session           *sessionContext // The session instance used for this session parameter - will be nil for a password authorization
	associatedContext ResourceContext // The resource associated with an authorization - can be nil
	includeAuthValue  bool            // Whether the authorization value of associatedContext is included in the HMAC key

	decryptNonce Nonce
	encryptNonce Nonce
}

func (s *sessionParam) IsAuth() bool {
	return s.associatedContext != nil
}

func (s *sessionParam) ComputeSessionHMACKey() []byte {
	var key []byte
	key = append(key, s.session.Data().SessionKey...)
	if s.includeAuthValue {
		key = append(key, s.associatedContext.(resourceContextPrivate).GetAuthValue()...)
	}
	return key
}

func (s *sessionParam) computeHMAC(pHash []byte, nonceNewer, nonceOlder, nonceDecrypt, nonceEncrypt Nonce, attrs SessionAttributes) ([]byte, bool) {
	key := s.ComputeSessionHMACKey()
	h := hmac.New(func() hash.Hash { return s.session.Data().HashAlg.NewHash() }, key)

	h.Write(pHash)
	h.Write(nonceNewer)
	h.Write(nonceOlder)
	h.Write(nonceDecrypt)
	h.Write(nonceEncrypt)
	h.Write([]byte{uint8(attrs)})

	return h.Sum(nil), len(key) > 0
}

func (s *sessionParam) computeCommandHMAC(commandCode CommandCode, commandHandles []Name, cpBytes []byte) []byte {
	data := s.session.Data()
	cpHash := ComputeCpHash(data.HashAlg, commandCode, commandHandles, cpBytes)
	h, _ := s.computeHMAC(cpHash, data.NonceCaller, data.NonceTPM, s.decryptNonce, s.encryptNonce, s.session.attrs.canonicalize())
	return h
}

func (s *sessionParam) buildCommandSessionAuth(commandCode CommandCode, commandHandles []Name, cpBytes []byte) *AuthCommand {
	data := s.session.Data()

	var hmac []byte

	if data.SessionType == SessionTypePolicy && data.PolicyHMACType == policyHMACTypePassword {
		// Policy session that contains a TPM2_PolicyPassword assertion. The HMAC is just the authorization value
		// of the resource being authorized.
		if s.IsAuth() {
			hmac = s.associatedContext.(resourceContextPrivate).GetAuthValue()
		}
	} else {
		hmac = s.computeCommandHMAC(commandCode, commandHandles, cpBytes)
	}

	return &AuthCommand{
		SessionHandle:     s.session.Handle(),
		Nonce:             data.NonceCaller,
		SessionAttributes: s.session.attrs.canonicalize(),
		HMAC:              hmac}
}

func (s *sessionParam) buildCommandPasswordAuth() *AuthCommand {
	return &AuthCommand{SessionHandle: HandlePW, SessionAttributes: AttrContinueSession, HMAC: s.associatedContext.(resourceContextPrivate).GetAuthValue()}
}

func (s *sessionParam) buildCommandAuth(commandCode CommandCode, commandHandles []Name, cpBytes []byte) *AuthCommand {
	if s.session == nil {
		// Cleartext password session
		return s.buildCommandPasswordAuth()
	}
	// HMAC or policy session
	return s.buildCommandSessionAuth(commandCode, commandHandles, cpBytes)
}

func (s *sessionParam) computeResponseHMAC(resp AuthResponse, responseCode ResponseCode, commandCode CommandCode, rpBytes []byte) ([]byte, bool) {
	data := s.session.Data()
	rpHash := cryptComputeRpHash(data.HashAlg, responseCode, commandCode, rpBytes)
	return s.computeHMAC(rpHash, data.NonceTPM, data.NonceCaller, nil, nil, resp.SessionAttributes)
}

func (s *sessionParam) processResponseAuth(resp AuthResponse, responseCode ResponseCode, commandCode CommandCode, rpBytes []byte) error {
	if s.session == nil {
		return nil
	}

	data := s.session.Data()
	data.NonceTPM = resp.Nonce
	data.IsAudit = resp.SessionAttributes&AttrAudit > 0
	data.IsExclusive = resp.SessionAttributes&AttrAuditExclusive > 0

	if data.SessionType == SessionTypePolicy && data.PolicyHMACType == policyHMACTypePassword {
		if len(resp.HMAC) != 0 {
			return errors.New("non-zero length HMAC for policy session with PolicyPassword assertion")
		}
		return nil
	}

	hmac, hmacRequired := s.computeResponseHMAC(resp, responseCode, commandCode, rpBytes)
	if (hmacRequired || len(resp.HMAC) > 0) && !bytes.Equal(hmac, resp.HMAC) {
		return errors.New("incorrect HMAC")
	}

	return nil
}

func computeBindName(name Name, auth Auth) Name {
	if len(auth) > len(name) {
		auth = auth[0:len(name)]
	}
	r := make(Name, len(name))
	copy(r, name)
	j := 0
	for i := len(name) - len(auth); i < len(name); i++ {
		r[i] ^= auth[j]
		j++
	}
	return r
}

type sessionParams struct {
	commandCode CommandCode
	sessions    []*sessionParam
}

func (p *sessionParams) findSessionWithAttr(attr SessionAttributes) (*sessionParam, int) {
	for i, session := range p.sessions {
		if session.session == nil {
			continue
		}
		if session.session.attrs.canonicalize()&attr > 0 {
			return session, i
		}
	}

	return nil, 0
}

func (p *sessionParams) validateAndAppend(s *sessionParam) error {
	if len(p.sessions) >= 3 {
		return errors.New("too many session parameters")
	}

	if s.session != nil {
		data := s.session.Data()
		if data == nil {
			return errors.New("invalid context for session: incomplete session can only be used in TPMContext.FlushContext")
		}
		switch data.SessionType {
		case SessionTypeHMAC:
			switch {
			case !s.IsAuth():
				// HMAC session not used for authorization
			case !data.IsBound:
				// A non-bound HMAC session used for authorization. Include the auth value of the associated
				// ResourceContext in the HMAC key
				s.includeAuthValue = true
			default:
				// A bound HMAC session used for authorization. Include the auth value of the associated
				// ResourceContext only if it is not the bind entity.
				bindName := computeBindName(s.associatedContext.Name(), s.associatedContext.(resourceContextPrivate).GetAuthValue())
				s.includeAuthValue = !bytes.Equal(bindName, data.BoundEntity)
			}
		case SessionTypePolicy:
			// A policy session that includes a TPM2_PolicyAuthValue assertion. Include the auth value of the associated
			// ResourceContext.
			switch {
			case !s.IsAuth():
				// This is actually an invalid case, but just let the TPM return the appropriate error
			default:
				s.includeAuthValue = data.PolicyHMACType == policyHMACTypeAuth
			}
		}
	}

	p.sessions = append(p.sessions, s)
	return nil
}

func (p *sessionParams) validateAndAppendAuth(in ResourceContextWithSession) error {
	sc, _ := in.Session.(*sessionContext)
	associatedContext := in.Context
	if associatedContext == nil {
		associatedContext = makePermanentContext(HandleNull)
	}
	s := &sessionParam{associatedContext: associatedContext, session: sc}
	return p.validateAndAppend(s)
}

func (p *sessionParams) validateAndAppendExtra(in []SessionContext) error {
	for _, s := range in {
		if s == nil {
			continue
		}
		if err := p.validateAndAppend(&sessionParam{session: s.(*sessionContext)}); err != nil {
			return err
		}
	}

	return nil
}

func (p *sessionParams) computeCallerNonces() error {
	for _, s := range p.sessions {
		if s.session == nil {
			continue
		}

		if err := cryptComputeNonce(s.session.Data().NonceCaller); err != nil {
			return fmt.Errorf("cannot compute new caller nonce: %v", err)
		}
	}
	return nil
}

func (p *sessionParams) buildCommandAuthArea(commandCode CommandCode, commandHandles []Name, cpBytes []byte) ([]AuthCommand, error) {
	if err := p.computeCallerNonces(); err != nil {
		return nil, fmt.Errorf("cannot compute caller nonces: %v", err)
	}

	if err := p.encryptCommandParameter(cpBytes); err != nil {
		return nil, fmt.Errorf("cannot encrypt first command parameter: %v", err)
	}

	p.computeEncryptNonce()
	p.commandCode = commandCode

	var area []AuthCommand
	for _, s := range p.sessions {
		a := s.buildCommandAuth(commandCode, commandHandles, cpBytes)
		area = append(area, *a)
	}

	return area, nil
}

func (p *sessionParams) invalidateSessionContexts(authResponses []AuthResponse) {
	for i, resp := range authResponses {
		session := p.sessions[i].session
		if session == nil {
			continue
		}
		if resp.SessionAttributes&AttrContinueSession != 0 {
			continue
		}
		session.invalidate()
	}
}

func (p *sessionParams) processResponseAuthArea(authResponses []AuthResponse, responseCode ResponseCode, rpBytes []byte) error {
	defer p.invalidateSessionContexts(authResponses)

	for i, resp := range authResponses {
		if err := p.sessions[i].processResponseAuth(resp, responseCode, p.commandCode, rpBytes); err != nil {
			return fmt.Errorf("encountered an error for session at index %d: %v", i, err)
		}
	}

	if err := p.decryptResponseParameter(rpBytes); err != nil {
		return fmt.Errorf("cannot decrypt first response parameter: %v", err)
	}

	return nil
}
