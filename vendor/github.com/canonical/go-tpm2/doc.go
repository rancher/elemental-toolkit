/*
Package tpm2 implements an API for communicating with TPM 2.0 devices.

This documentation refers to TPM commands and types that are described in more detail in the TPM 2.0 Library Specification, which can
be found at https://trustedcomputinggroup.org/resource/tpm-library-specification/. Knowledge of this specification is assumed in this
documentation.

Communication with Linux TPM character devices and TPM simulators implementing the Microsoft TPM2 simulator interface is supported.
The core type by which consumers of this package communicate with a TPM is TPMContext.

Quick start

In order to create a new TPMContext that can be used to communicate with a Linux TPM character device:
 tcti, err := linux.OpenDevice("/dev/tpm0")
 if err != nil {
	 return err
 }
 tpm := tpm2.NewTPMContext(tcti)

In order to create and persist a new storage primary key:
 tcti, err := linux.OpenDevice("/dev/tpm0")
 if err != nil {
	return err
 }
 tpm := tpm2.NewTPMContext(tcti)

 template = tpm2.Public{
	Type:    tpm2.ObjectTypeRSA,
	NameAlg: tpm2.HashAlgorithmSHA256,
	Attrs: tpm2.AttrFixedTPM | tpm2.AttrFixedParent | tpm2.AttrSensitiveDataOrigin | tpm2.AttrUserWithAuth | tpm2.AttrNoDA | tpm2.AttrRestricted | tpm2.AttrDecrypt,
	Params: &tpm2.PublicParamsU{
		RSADetail: &tpm2.RSAParams{
			Symmetric: tpm2.SymDefObject{
				Algorithm: tpm2.SymObjectAlgorithmAES,
				KeyBits:   &tpm2.SymKeyBitsU{Sym: 128},
				Mode:      &tpm2.SymModeU{Sym: tpm2.SymModeCFB}},
			Scheme:   tpm2.RSAScheme{Scheme: tpm2.RSASchemeNull},
			KeyBits:  2048,
			Exponent: 0}},
	Unique: &tpm2.PublicIDU{RSA: make(tpm2.PublicKeyRSA, 256)}}
 context, _, _, _, _, err := tpm.CreatePrimary(tpm.OwnerHandleContext(), nil, &template, nil, nil, nil)
 if err != nil {
	return err
 }

 persistentContext, err := tpm.EvictControl(tpm.OwnerHandleContext(), context, tpm2.Handle(0x81000001), nil)
 if err != nil {
	return err
 }
 // persistentContext is a ResourceContext corresponding to the new persistent storage primary key.

In order to evict a persistent object:
 tcti, err := linux.OpenDevice("/dev/tpm0")
 if err != nil {
	return err
 }
 tpm := tpm2.NewTPMContext(tcti)

 context, err := tpm.CreateResourceContextFromTPM(tpm2.Handle(0x81000001))
 if err != nil {
	 return err
 }

 if _, err := tpm.EvictControl(tpm.OwnerHandleContext(), context, context.Handle(), nil); err != nil {
	 return err
 }
 // The resource associated with context is now unavailable.

Authorization types

Some TPM resources require authorization in order to use them in some commands. There are 3 main types of authorization supported by
this package:
 * Cleartext password: A cleartext authorization value is sent to the TPM by calling ResourceContext.SetAuthValue and supplying the
 ResourceContext to a function requiring authorization. Authorization succeeds if the correct value is sent.

 * HMAC session: Knowledge of an authorization value is demonstrated by calling ResourceContext.SetAuthValue and supplying the ResourceContext
 to a function requiring authorization, along with a session with the type SessionTypeHMAC. Authorization succeeds if the computed HMAC
 matches that expected by the TPM.

 * Policy session: A ResourceContext is supplied to a function requiring authorization along with a session with the type
 SessionTypePolicy, containing a record of and the result of a sequence of assertions. Authorization succeeds if the conditions
 required by the resource's authorization policy are satisfied.

The type of authorizations permitted for a resource is dependent on the authorization role (user, admin or duplication), the type of
resource and the resource's attributes.
*/
package tpm2
