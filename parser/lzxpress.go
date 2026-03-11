package parser

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func readU32(r io.Reader) (uint32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	return binary.LittleEndian.Uint32(buf[:]), err
}

func readU16(r io.Reader) (uint16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	return binary.LittleEndian.Uint16(buf[:]), err
}

func readByte(r io.Reader) (byte, error) {
	var buf [1]byte
	_, err := io.ReadFull(r, buf[:])
	return buf[0], err
}

// DecompressLZXCompression decompresses LZXPRESS data from a byte slice.
func DecompressLZXCompression(src []byte) ([]byte, error) {
	r := bytes.NewReader(src)
	size := int64(len(src))
	dst := make([]byte, 0, size*2)

	var flags uint32
	var flagCount int
	var lastHalfByte int64

	for pos := int64(0); pos < size; {
		if flagCount == 0 {
			var err error
			flags, err = readU32(r)
			if err != nil {
				return nil, fmt.Errorf("read flags: %w", err)
			}
			flagCount = 32
		}
		flagCount--
		pos += 4

		if flags&(1<<flagCount) == 0 {
			b, err := readByte(r)
			if err != nil {
				return nil, fmt.Errorf("read literal: %w", err)
			}
			dst, pos = append(dst, b), pos+1
			continue
		}

		if pos >= size {
			break
		}

		match, err := readU16(r)
		if err != nil {
			return nil, fmt.Errorf("read match: %w", err)
		}
		pos += 2

		offset := int(match/8) + 1
		length := int(match % 8)

		if length == 7 {
			if lastHalfByte == 0 {
				lastHalfByte = pos
				b, _ := readByte(r)
				length = int(b % 16)
				pos++
			} else {
				cur, _ := r.Seek(0, io.SeekCurrent)
				_, _ = r.Seek(lastHalfByte, io.SeekStart)
				b, _ := readByte(r)
				_, _ = r.Seek(cur, io.SeekStart)
				length = int(b / 16)
				lastHalfByte = 0
			}

			if length == 15 {
				b, _ := readByte(r)
				pos++
				length = int(b)

				if length == 255 {
					u16, _ := readU16(r)
					pos += 2
					length = int(u16)
					if length == 0 {
						u32, _ := readU32(r)
						pos += 4
						length = int(u32)
					}
					if length < 22 {
						return nil, errors.New("wrong match length")
					}
					length -= 22
				}
				length += 15
			}
			length += 7
		}
		length += 3

		for remaining := length; remaining > 0; {
			n := remaining
			if n > offset {
				n = offset
			}
			start := len(dst) - offset
			for i := 0; i < n; i++ {
				dst = append(dst, dst[start+i])
			}
			remaining -= n
		}
	}

	return dst, nil
}
