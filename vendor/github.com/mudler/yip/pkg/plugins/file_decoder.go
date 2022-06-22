package plugins

type decoder interface {
	Decode(content string) ([]byte, error)
}

func newDecoder(s string) decoder {
	switch s {
	case "b64", "base64":
		return base64Decoder{}
	case "gz", "gzip":
		return gzipDecoder{}
	case "gz+base64", "gzip+base64", "gz+b64", "gzip+b64":
		return base64GZip{}
	default:
		return byteDecoder{}
	}
}

type byteDecoder struct {
}

func (byteDecoder) Decode(content string) ([]byte, error) {
	return []byte(content), nil
}
