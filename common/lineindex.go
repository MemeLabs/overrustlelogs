package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"strings"
)

// ErrInvalidLineLength corrupt index
var ErrInvalidLineLength = errors.New("invalid line length")

// ReadLineIndex adds index from the disk
func ReadLineIndex(n *LineIndex, path string) error {
	buf, err := ReadCompressedFile(path)
	if err != nil {
		return err
	}
	offset := 0
	for {
		v, size := binary.Uvarint(buf[offset:])
		if size < 0 {
			return ErrInvalidLineLength
		}
		n.Add(uint16(v))
		offset += size
		if offset >= len(buf) {
			return nil
		}
	}
	return nil
}

// LineIndex index of line lengths
type LineIndex []uint16

// Add append line length
func (n *LineIndex) Add(length uint16) {
	*n = append(*n, length)
}

// WriteTo writes index to the disk
func (n *LineIndex) WriteTo(path string) error {
	buf := bytes.NewBuffer([]byte{})
	temp := make([]byte, binary.MaxVarintLen16)
	for _, length := range *n {
		size := binary.PutUvarint(temp, uint64(length))
		buf.Write(temp[:size])
	}
	f, err := WriteCompressedFile(path+".writing", buf.Bytes())
	if err != nil {
		return err
	}
	if err := os.Rename(f.Name(), strings.Replace(f.Name(), ".writing", "", -1)); err != nil {
		return err
	}
	return nil
}
