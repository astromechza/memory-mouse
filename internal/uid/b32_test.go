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
			n, err := EncodeCB32(sb, strings.NewReader(in))
			if testsupport.AssertEqual(t, err, nil) {
				testsupport.AssertEqual(t, n, int64(sb.Len()))
				testsupport.AssertEqual(t, sb.String(), out)
			}
			o2, err := EncodeCB32String([]byte(in))
			if testsupport.AssertEqual(t, err, nil) {
				testsupport.AssertEqual(t, o2, out)
			}
		})
		t.Run("decode", func(t *testing.T) {
			sb.Reset()
			n, err := DecodeCB32(sb, strings.NewReader(out))
			if testsupport.AssertEqual(t, err, nil) {
				testsupport.AssertEqual(t, n, int64(sb.Len()))
				testsupport.AssertEqual(t, sb.String(), in)
			}
			o2, err := DecodeCB32String(out)
			if testsupport.AssertEqual(t, err, nil) {
				testsupport.AssertEqual(t, string(o2), in)
			}
		})
	})
}

func TestEncodeDecodeCB32(t *testing.T) {
	roundTripTest(t, "empty", "", "")
	roundTripTest(t, "1byte", "a", "C4")
	roundTripTest(t, "2byte", "ab", "C5H0")
	roundTripTest(t, "3byte", "abc", "C5H66")
	roundTripTest(t, "4byte", "abcd", "C5H66S0")
}

func FuzzEncodeDecodeCB32(f *testing.F) {
	for _, s := range []string{"", "a", "zzzzz", ".", " ", "\n", "🎃", "\x00\xff", strings.Repeat("x", 100)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		sb := new(bytes.Buffer)
		n, err := EncodeCB32(sb, strings.NewReader(in))
		if testsupport.AssertEqual(t, err, nil) {
			testsupport.AssertEqual(t, n, int64(sb.Len()))
		}
		out := sb.String()
		sb.Reset()
		n, err = DecodeCB32(sb, strings.NewReader(out))
		if testsupport.AssertEqual(t, err, nil) {
			testsupport.AssertEqual(t, n, int64(sb.Len()))
			testsupport.AssertEqual(t, sb.String(), in)
		}
	})
}
