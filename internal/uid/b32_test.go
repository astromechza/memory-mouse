package uid

import (
	"bytes"
	"strings"
	"testing"

	"github.com/astromechza/memory-mouse/internal/testsupport"
)

func roundTripTest(t *testing.T, name, in, out string) {
	t.Run(name, func(t *testing.T) {
		sb := new(bytes.Buffer)
		t.Run("encode", func(t *testing.T) {
			n, err := EncodeB32(sb, strings.NewReader(in))
			if testsupport.AssertEqual(t, err, nil) {
				testsupport.AssertEqual(t, n, int64(sb.Len()))
				testsupport.AssertEqual(t, sb.String(), out)
			}
		})
		t.Run("decode", func(t *testing.T) {
			sb.Reset()
			n, err := DecodeB32(sb, strings.NewReader(out))
			if testsupport.AssertEqual(t, err, nil) {
				testsupport.AssertEqual(t, n, int64(sb.Len()))
				testsupport.AssertEqual(t, sb.String(), in)
			}
		})
	})
}

func TestEncodeDecodeB32(t *testing.T) {
	roundTripTest(t, "empty", "", "")
	roundTripTest(t, "1byte", "a", "C4")
	roundTripTest(t, "2byte", "ab", "C5H0")
	roundTripTest(t, "3byte", "abc", "C5H66")
	roundTripTest(t, "4byte", "abcd", "C5H66S0")
}
