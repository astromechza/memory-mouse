package uid

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// bitPump is the core of the bit encoder and decoder. It reads bytes from the src, decodes them into integer chunks and
// adds them to the buffer. Whenever the buffer has enough content for an output chunk, we read it, convert it to a byte
// and then write it.
func bitPump(src io.ByteReader, srcDecoder map[byte]int, srcChunkSize int, dropExtra bool, dstChunkSize int, dstEncoder map[int]byte, dst io.ByteWriter) (int64, error) {
	// Setup some initial assignments
	rem, remBits, i, b, ok, written, err := 0, 0, 0, byte(0), false, int64(0), error(nil)
	// Now loop through the main body of the src stream. Reading bytes to fill in the buffer until we have enough for an output chunk.
	for {
		// If we have enough bits for an output chunk, let's produce one.
		if remBits >= dstChunkSize {
			// extract a dst chunk
			remBits -= dstChunkSize
			i = rem >> remBits
			rem = rem & ((1 << remBits) - 1)
			// encode it by converting the chunk to an output byte, if no alphabet is defined then just cast it.
			if dstEncoder != nil {
				if b, ok = dstEncoder[i]; !ok {
					return written, fmt.Errorf("unknown dst alphabet: %v", b)
				}
			} else {
				b = byte(i)
			}
			// write the byte
			if err = dst.WriteByte(b); err != nil {
				return written, err
			}
			written += 1
		} else {
			// if we don't have enough data, read a chunk from the input stream
			if b, err = src.ReadByte(); err != nil {
				// If we have an EOF here, then we have to break, because we know we don't have enough
				// for a complete chunk.
				if errors.Is(err, io.EOF) {
					break
				}
				return written, err
			} else {
				// now convert it into it's integer chunk or just cast it if no decoder is specified
				if srcDecoder != nil {
					i, ok = srcDecoder[b]
					if !ok {
						return written, fmt.Errorf("unknown src alphabet: %v", b)
					}
				} else {
					i = int(b)
				}
				// add it to the buffer
				rem = (rem << srcChunkSize) | i
				remBits += srcChunkSize
			}
		}
	}
	// Now if we are left with some bits in the buffer, we either need to pad them with 0's in order to produce enough
	// content, or we must just drop it if it's nil data (usually during decoding).
	if remBits > 0 && (!dropExtra || rem > 0) {
		// read the chunk
		i = rem << (dstChunkSize - remBits)
		// convert it to an output byte using the encoder or cast
		if dstEncoder != nil {
			b, ok = dstEncoder[i]
			if !ok {
				return written, fmt.Errorf("unknown dst alphabet: %v", b)
			}
		} else {
			b = byte(i)
		}
		if err = dst.WriteByte(b); err != nil {
			return written, err
		}
		written += 1
	}
	return written, nil
}

var cb32Encoding = map[int]byte{
	0: '0', 1: '1', 2: '2', 3: '3', 4: '4', 5: '5', 6: '6', 7: '7',
	8: '8', 9: '9', 10: 'A', 11: 'B', 12: 'C', 13: 'D', 14: 'E', 15: 'F',
	16: 'G', 17: 'H', 18: 'J', 19: 'K', 20: 'M', 21: 'N', 22: 'P', 23: 'Q',
	24: 'R', 25: 'S', 26: 'T', 27: 'V', 28: 'W', 29: 'X', 30: 'Y', 31: 'Z',
}

var cb32Decoding map[byte]int

func init() {
	cb32Decoding = make(map[byte]int, len(cb32Encoding))
	cb32Decoding['O'] = 0
	cb32Decoding['I'] = 1
	cb32Decoding['L'] = 1
	for i, b := range cb32Encoding {
		cb32Decoding[b] = i
		if b >= 'A' && b <= 'Z' {
			cb32Decoding[b+32] = i
		}
	}
}

func EncodeCB32(dst io.ByteWriter, src io.ByteReader) (written int64, err error) {
	return bitPump(src, nil, 8, false, 5, cb32Encoding, dst)
}

func DecodeCB32(dst io.ByteWriter, src io.ByteReader) (written int64, err error) {
	return bitPump(src, cb32Decoding, 5, true, 8, nil, dst)
}

func EncodeCB32String(in []byte) (string, error) {
	sb := bytes.NewBuffer(make([]byte, 0, len(in)*2))
	if _, err := EncodeCB32(sb, bytes.NewReader(in)); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func DecodeCB32String(in string) ([]byte, error) {
	sb := bytes.NewBuffer(make([]byte, 0, len(in)))
	if _, err := DecodeCB32(sb, strings.NewReader(in)); err != nil {
		return nil, err
	}
	return sb.Bytes(), nil
}
