package common

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/cloudflare/golz4"
)

// WriteCompressedFile write compressed file
func WriteCompressedFile(path string, data []byte) (*os.File, error) {
	if path[len(path)-4:] != ".lz4" {
		path += ".lz4"
	}
	c := make([]byte, lz4.CompressBound(data)+4)
	size, err := lz4.CompressHC(data, c[4:])
	c[0] = byte(len(data) >> 24)
	c[1] = byte(len(data) >> 16)
	c[2] = byte(len(data) >> 8)
	c[3] = byte(len(data))
	size += 4
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(c[:size]); err != nil {
		return nil, err
	}
	return f, nil
}

// ReadCompressedFile read compressed file
func ReadCompressedFile(path string) ([]byte, error) {
	if path[len(path)-4:] != ".lz4" {
		path += ".lz4"
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	c, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	size := uint32(0)
	size |= uint32(c[0]) << 24
	size |= uint32(c[1]) << 16
	size |= uint32(c[2]) << 8
	size |= uint32(c[3])
	data := make([]byte, size)
	if err := lz4.Uncompress(c[4:], data); err != nil {
		return nil, err
	}
	return data, nil
}

// CompressFile compress an existing file
func CompressFile(path string) (*os.File, error) {
	s, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(s)
	s.Close()
	if err != nil {
		return nil, err
	}
	d, err := WriteCompressedFile(path, data)
	if err != nil {
		return nil, err
	}
	os.Remove(path)
	return d, nil
}

// UncompressFile compress an existing file
func UncompressFile(path string) (*os.File, error) {
	d, err := ReadCompressedFile(path)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(strings.Replace(path, ".lz4", "", -1), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(d); err != nil {
		return nil, err
	}
	os.Remove(path)
	return f, nil
}
