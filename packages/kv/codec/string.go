package codec

import "errors"

func DecodeString(b []byte, def ...string) (string, error) {
	if b == nil {
		if len(def) == 0 {
			return "", errors.New("cannot decode nil string")
		}
		return def[0], nil
	}
	return string(b), nil
}

func EncodeString(value string) []byte {
	return []byte(value)
}
