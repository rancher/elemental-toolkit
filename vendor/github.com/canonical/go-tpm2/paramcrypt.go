// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

import (
	"crypto/aes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/canonical/go-tpm2/internal"
	"github.com/canonical/go-tpm2/mu"
)

func isParamEncryptable(param interface{}) bool {
	return mu.DetermineTPMKind(param) == mu.TPMKindSized
}

func (s *sessionParam) computeSessionValue() []byte {
	var key []byte
	key = append(key, s.session.Data().SessionKey...)
	if s.IsAuth() {
		key = append(key, s.associatedContext.(resourceContextPrivate).GetAuthValue()...)
	}
	return key
}

func (p *sessionParams) findDecryptSession() (*sessionParam, int) {
	return p.findSessionWithAttr(AttrCommandEncrypt)
}

func (p *sessionParams) findEncryptSession() (*sessionParam, int) {
	return p.findSessionWithAttr(AttrResponseEncrypt)
}

func (p *sessionParams) hasDecryptSession() bool {
	s, _ := p.findDecryptSession()
	return s != nil
}

func (p *sessionParams) computeEncryptNonce() {
	s, i := p.findEncryptSession()
	if s == nil || i == 0 || !p.sessions[0].IsAuth() {
		return
	}
	ds, di := p.findDecryptSession()
	if ds != nil && di == i {
		return
	}

	p.sessions[0].encryptNonce = s.session.NonceTPM()
}

func (p *sessionParams) encryptCommandParameter(cpBytes []byte) error {
	s, i := p.findDecryptSession()
	if s == nil {
		return nil
	}

	sessionData := s.session.Data()
	hashAlg := sessionData.HashAlg

	sessionValue := s.computeSessionValue()

	size := binary.BigEndian.Uint16(cpBytes)
	data := cpBytes[2 : size+2]

	symmetric := sessionData.Symmetric

	switch symmetric.Algorithm {
	case SymAlgorithmAES:
		if symmetric.Mode.Sym != SymModeCFB {
			return errors.New("unsupported cipher mode")
		}
		k := internal.KDFa(hashAlg.GetHash(), sessionValue, []byte(CFBKey), sessionData.NonceCaller, sessionData.NonceTPM,
			int(symmetric.KeyBits.Sym)+(aes.BlockSize*8))
		offset := (symmetric.KeyBits.Sym + 7) / 8
		symKey := k[0:offset]
		iv := k[offset:]
		if err := CryptSymmetricEncrypt(symmetric.Algorithm, symKey, iv, data); err != nil {
			return fmt.Errorf("AES encryption failed: %v", err)
		}
	case SymAlgorithmXOR:
		internal.XORObfuscation(hashAlg.GetHash(), sessionValue, sessionData.NonceCaller, sessionData.NonceTPM, data)
	default:
		return fmt.Errorf("unknown symmetric algorithm: %v", symmetric.Algorithm)
	}

	if i > 0 && p.sessions[0].IsAuth() {
		p.sessions[0].decryptNonce = sessionData.NonceTPM
	}

	return nil
}

func (p *sessionParams) decryptResponseParameter(rpBytes []byte) error {
	s, _ := p.findEncryptSession()
	if s == nil {
		return nil
	}

	sessionData := s.session.Data()
	hashAlg := sessionData.HashAlg

	sessionValue := s.computeSessionValue()

	size := binary.BigEndian.Uint16(rpBytes)
	data := rpBytes[2 : size+2]

	symmetric := sessionData.Symmetric

	switch symmetric.Algorithm {
	case SymAlgorithmAES:
		if symmetric.Mode.Sym != SymModeCFB {
			return errors.New("unsupported cipher mode")
		}
		k := internal.KDFa(hashAlg.GetHash(), sessionValue, []byte(CFBKey), sessionData.NonceTPM, sessionData.NonceCaller,
			int(symmetric.KeyBits.Sym)+(aes.BlockSize*8))
		offset := (symmetric.KeyBits.Sym + 7) / 8
		symKey := k[0:offset]
		iv := k[offset:]
		if err := CryptSymmetricDecrypt(symmetric.Algorithm, symKey, iv, data); err != nil {
			return fmt.Errorf("AES encryption failed: %v", err)
		}
	case SymAlgorithmXOR:
		internal.XORObfuscation(hashAlg.GetHash(), sessionValue, sessionData.NonceTPM, sessionData.NonceCaller, data)
	default:
		return fmt.Errorf("unknown symmetric algorithm: %v", symmetric.Algorithm)
	}

	return nil
}
