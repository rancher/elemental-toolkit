/*
Package mu provides helpers to marshalling to and unmarshalling from the TPM wire format.

Go types are marshalled to and from the TPM wire format according to the following rules:
 * UINT8 <-> uint8
 * BYTE <-> byte
 * INT8 <-> int8
 * BOOL <-> bool
 * UINT16 <-> uint16
 * INT16 <-> int16
 * UINT32 <-> uint32
 * INT32 <-> int32
 * UINT64 <-> uint64
 * INT64 <-> int64
 * TPM2B prefixed types (sized buffers with a 2-byte size field) fall in to 2 categories:
    * Byte buffer <-> []byte, or any type with an identical underlying type.
    * Sized structure <-> struct referenced from a pointer field in another structure, where the field has the `tpm2:"sized"`
      tag. A zero sized struct is represented as a nil pointer.
 * TPMA prefixed types (attributes) <-> whichever go type corresponds to the underlying TPM type (UINT8, UINT16, or UINT32).
 * TPM_ALG_ID (algorithm enum) <-> tpm2.AlgorithmId
 * TPML prefixed types (lists with a 4-byte length field) <-> slice of whichever go type corresponds to the underlying TPM type.
 * TPMS prefixed types (structures) <-> struct
 * TPMT prefixed types (structures with a tag field used as a union selector) <-> struct
 * TPMU prefixed types (unions) <-> struct which implements the Union interface. These must be contained in another structure
   and referenced from a pointer field. The first field of the enclosing structure is used as the selector value, although this
   can be overridden by using the `tpm2:"selector:<field_name>"` tag.

TPMI prefixed types (interface types) are generally not explicitly supported. These are used by the TPM for type checking during
unmarshalling, but this package doesn't distinguish between TPMI prefixed types with the same underlying type.

The marshalling code parses the "tpm2" tag on struct fields, the value of which is a comma separated list of options. These options are:
 * selector:<field_name> - used on fields that are pointers to structs that implement the Union interface to specify the field
 used as the selector value. The default behaviour without this option is to use the first field as the selector.
 * sized - used on fields that are pointers to indicate that it should be marshalled and unmarshalled as a sized value.
 A nil pointer represents a zero-sized value. This is used to implement sized structures.
 * raw - used on slice fields to indicate that it should be marshalled and unmarshalled without a length (if it represents a list)
 or size (if it represents a sized buffer) field. The slice must be pre-allocated to the correct length by the caller during
 unmarshalling.
*/
package mu
