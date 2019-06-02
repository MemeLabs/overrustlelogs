package common

import (
	"bytes"
	"fmt"
	"io"

	"github.com/actgardner/gogen-avro/container"
)

// WriterConstructor ...
type WriterConstructor func(writer io.Writer, codec container.Codec, recordsPerBlock int64) (*container.Writer, error)

// AvroBuffer ...
type AvroBuffer struct {
	WriterConstructor WriterConstructor
	Codec             container.Codec
	RecordsPerBlock   int64
	BytesPerFile      int
	Writer            io.Writer
	avroWriter        *container.Writer
	buffer            bytes.Buffer
	recordCount       int64
}

// NewAvroBuffer ...
func NewAvroBuffer(writerConstructor WriterConstructor, writer io.Writer, codec container.Codec, recordsPerBlock int64, bytesPerFile int) (*AvroBuffer, error) {
	a := &AvroBuffer{
		WriterConstructor: writerConstructor,
		Codec:             codec,
		RecordsPerBlock:   recordsPerBlock,
		BytesPerFile:      bytesPerFile,
		Writer:            writer,
	}

	if err := a.initAvroWriter(); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *AvroBuffer) initAvroWriter() (err error) {
	a.avroWriter, err = a.WriterConstructor(&a.buffer, a.Codec, a.RecordsPerBlock)
	if err != nil {
		return fmt.Errorf("initializing avro writer: %v", err)
	}

	return
}

// WriteRecord ...
func (a *AvroBuffer) WriteRecord(record container.AvroRecord) error {
	if err := a.avroWriter.WriteRecord(record); err != nil {
		return err
	}

	a.recordCount++

	if a.buffer.Len() >= a.BytesPerFile {
		if err := a.Flush(); err != nil {
			return fmt.Errorf("writing record: %v", err)
		}
	}

	return nil
}

// Flush ...
func (a *AvroBuffer) Flush() error {
	if a.recordCount%a.RecordsPerBlock != 0 {
		if err := a.avroWriter.Flush(); err != nil {
			return fmt.Errorf("flushing avroWriter: %v", err)
		}
	}

	if _, err := a.Writer.Write(a.buffer.Bytes()); err != nil {
		return fmt.Errorf("flushing buffer: %v", err)
	}
	a.buffer.Reset()

	if err := a.initAvroWriter(); err != nil {
		return fmt.Errorf("reinitializing writer: %v", err)
	}

	a.recordCount = 0

	return nil
}
