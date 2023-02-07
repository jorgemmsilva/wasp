package util

import "github.com/near/borsh-go"

func MustSerialize(obj interface{}) []byte {
	ret, err := borsh.Serialize(obj)
	if err != nil {
		panic(err)
	}
	return ret
}

func Deserialize[T any](data []byte) (T, error) {
	s := new(T)
	err := borsh.Deserialize(s, data)
	return *s, err
}

func MustDeserialize[T any](data []byte) T {
	ret, err := Deserialize[T](data)
	if err != nil {
		panic(err)
	}
	return ret
}
