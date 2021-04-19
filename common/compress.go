package common

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/DataDog/zstd"
)

// WriteCompressedFile write compressed file
func WriteCompressedFile(path string, data []byte) (*os.File, error) {
	cData, err := zstd.Compress(nil, data)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(gzPath(path), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Write(cData); err != nil {
		return nil, err
	}
	return f, nil
}

// ReadCompressedFile read compressed file
func ReadCompressedFile(path string) ([]byte, error) {
	f, err := os.Open(gzPath(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	dData, err := zstd.Decompress(nil, data)
	if err != nil {
		return nil, err
	}
	return dData, nil
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
	if err := os.Remove(path); err != nil {
		return nil, err
	}
	return d, nil
}

// UncompressFile uncompress an existing file
func UncompressFile(path string) (*os.File, error) {
	d, err := ReadCompressedFile(path)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(strings.Replace(path, ".gz", "", -1), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(d); err != nil {
		return nil, err
	}
	if err := os.Remove(gzPath(path)); err != nil {
		return nil, err
	}
	return f, nil
}

func gzPath(path string) string {
	if path[len(path)-3:] != ".gz" {
		path += ".gz"
	}
	return path
}
