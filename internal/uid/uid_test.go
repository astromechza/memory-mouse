package uid

import (
	"testing"

	"github.com/astromechza/memory-mouse/internal/testsupport"
)

func TestDocumentUid(t *testing.T) {
	t.Log(DocumentUid())
	testsupport.AssertEqual(t, len(DocumentUid()), 24)
}

func TestDocumentUidWithSource_empty(t *testing.T) {
	testsupport.AssertEqual(t, DocumentUidWithSource(readerFunc(func(b []byte) (n int, err error) {
		return len(b), nil
	})), "000000000000000000000000")
}

func TestDocumentUidWithSource_numeric(t *testing.T) {
	testsupport.AssertEqual(t, DocumentUidWithSource(readerFunc(func(b []byte) (n int, err error) {
		for i := range b {
			b[i] = byte(i)
		}
		return len(b), nil
	})), "000G40R40M30E209185GR38E")
}
