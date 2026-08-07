package projecttypeenvassertionreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

const (
	maximumCanonicalReportBytes = 64 << 20
	maximumCanonicalElements    = 1 << 20
)

type canonicalWriter struct {
	buffer bytes.Buffer
}

func newCanonicalWriter(domain string) canonicalWriter {
	writer := canonicalWriter{}
	writer.addString("haft.project-typeenv.canonical-envelope.v1")
	writer.addString(domain)
	return writer
}

func (writer *canonicalWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *canonicalWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.addBytes(encoded[:])
}

func (writer *canonicalWriter) addBytes(value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	writer.buffer.Write(length[:])
	writer.buffer.Write(value)
}

func (writer canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type canonicalReader struct {
	raw    []byte
	offset int
}

func newCanonicalReader(raw []byte, domain string) (*canonicalReader, error) {
	if len(raw) == 0 || len(raw) > maximumCanonicalReportBytes {
		return nil, fmt.Errorf("canonical value size is outside the supported bound")
	}
	reader := &canonicalReader{raw: append([]byte(nil), raw...)}
	envelope, err := reader.readString()
	if err != nil || envelope != "haft.project-typeenv.canonical-envelope.v1" {
		return nil, fmt.Errorf("canonical envelope is invalid")
	}
	actualDomain, err := reader.readString()
	if err != nil || actualDomain != domain {
		return nil, fmt.Errorf("canonical domain is invalid")
	}
	return reader, nil
}

func (reader *canonicalReader) readBytes() ([]byte, error) {
	if len(reader.raw)-reader.offset < 8 {
		return nil, fmt.Errorf("canonical field length is truncated")
	}
	length := binary.BigEndian.Uint64(reader.raw[reader.offset : reader.offset+8])
	reader.offset += 8
	remaining := len(reader.raw) - reader.offset
	if length > maximumCanonicalReportBytes {
		return nil, fmt.Errorf("canonical field is truncated")
	}
	boundedLength := int(length)
	if boundedLength > remaining {
		return nil, fmt.Errorf("canonical field is truncated")
	}
	end := reader.offset + boundedLength
	value := append(
		[]byte(nil),
		reader.raw[reader.offset:end]...,
	)
	reader.offset = end
	return value, nil
}

func (reader *canonicalReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("canonical string is not valid UTF-8")
	}
	return string(value), nil
}

func (reader *canonicalReader) readUint64() (uint64, error) {
	value, err := reader.readBytes()
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, fmt.Errorf("canonical uint64 has invalid width")
	}
	return binary.BigEndian.Uint64(value), nil
}

func (reader canonicalReader) requireEnd() error {
	if reader.offset != len(reader.raw) {
		return fmt.Errorf("canonical value has trailing bytes")
	}
	return nil
}

func canonicalDigest(raw []byte) [32]byte {
	return sha256.Sum256(raw)
}
