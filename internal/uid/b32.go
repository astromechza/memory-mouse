package uid

import (
	"fmt"
	"io"
)

var encodingAlphabet = map[int]byte{
	0: '0', 1: '1', 2: '2', 3: '3', 4: '4', 5: '5', 6: '6', 7: '7',
	8: '8', 9: '9', 10: 'A', 11: 'B', 12: 'C', 13: 'D', 14: 'E', 15: 'F',
	16: 'G', 17: 'H', 18: 'J', 19: 'K', 20: 'M', 21: 'N', 22: 'P', 23: 'Q',
	24: 'R', 25: 'S', 26: 'T', 27: 'V', 28: 'W', 29: 'X', 30: 'Y', 31: 'Z',
}

var decodingAlphabet map[byte]byte

func init() {
	decodingAlphabet = make(map[byte]byte, len(encodingAlphabet))
	decodingAlphabet['O'] = 0
	decodingAlphabet['I'] = 1
	decodingAlphabet['L'] = 1
	for i, b := range encodingAlphabet {
		decodingAlphabet[b] = byte(i)
		if b >= 'A' && b <= 'Z' {
			decodingAlphabet[b+32] = byte(i)
		}
	}
}

func EncodeB32(dst io.ByteWriter, src io.ByteReader) (written int64, err error) {
	rem, remBits := 0, 0
	for {
		if remBits < 5 {
			if b, err := src.ReadByte(); err != nil {
				if err == io.EOF {
					break
				}
				return written, err
			} else {
				rem = (rem << 8) | int(b)
				remBits += 8
			}
		}
		remBits -= 5
		ob := encodingAlphabet[rem>>remBits]
		rem = rem & ((1 << remBits) - 1)
		if err := dst.WriteByte(ob); err != nil {
			return written, err
		}
		written += 1
	}
	if remBits > 0 {
		if err := dst.WriteByte(encodingAlphabet[rem<<(5-remBits)]); err != nil {
			return written, err
		}
		written += 1
	}
	return written, nil
}

func DecodeB32(dst io.ByteWriter, src io.ByteReader) (written int64, err error) {
	rem, remBits := 0, 0
	for {
		// If we successfully have at least 8 bits then we can convert them. Otherwise, we return to our loop.
		if remBits >= 8 {
			remBits -= 8
			ob := byte(rem >> remBits)
			if err := dst.WriteByte(ob); err != nil {
				return written, err
			}
			rem = rem & ((1 << remBits) - 1)
			written += 1
		}

		// Each base32 byte we read passes through the decoding alphabet and produces 5 bits which we add to our
		// remaining bit set. We continue doing this until we have at least 8 bits or we reach EOF.
		if b, err := src.ReadByte(); err != nil {
			if err == io.EOF {
				break
			}
			return written, err
		} else {
			i, ok := decodingAlphabet[b]
			if !ok {
				return written, fmt.Errorf("unknown alphabet: %v", b)
			}
			rem = (rem << 5) | int(i)
			remBits += 5
		}
	}
	if remBits > 0 && rem > 0 {
		if err := dst.WriteByte(byte(rem)); err != nil {
			return written, err
		}
		written += 1
	}
	return written, nil
}
