package vault

import "encoding/base64"

func b64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func b64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
