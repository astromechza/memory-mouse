package uid

import (
	"crypto/rand"
	"io"

	"github.com/astromechza/memory-mouse/internal/cb32"
)

func DocumentUidWithSource(source io.Reader) string {
	b := make([]byte, 5*3)
	if _, err := source.Read(b); err != nil {
		panic(err)
	}
	o, _ := cb32.EncodeCB32String(b)
	return o
}

type readerFunc func(b []byte) (n int, err error)

func (f readerFunc) Read(b []byte) (n int, err error) {
	return f(b)
}

func DocumentUid() string {
	return DocumentUidWithSource(readerFunc(rand.Read))
}
