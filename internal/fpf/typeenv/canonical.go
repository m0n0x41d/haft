package typeenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const (
	canonicalDomain       = "haft.fpf.typeenv.canonical.v1"
	maximumCanonicalBytes = 20 << 20
	maximumCollectionSize = 64 << 10
)

type canonicalWriter struct {
	buffer bytes.Buffer
}

func newCanonicalWriter(domain string) canonicalWriter {
	writer := canonicalWriter{}
	writer.addString(canonicalDomain)
	writer.addString(domain)
	return writer
}

func (writer *canonicalWriter) addByte(value byte) {
	writer.buffer.WriteByte(value)
}

func (writer *canonicalWriter) addBool(value bool) {
	if value {
		writer.addByte(1)
		return
	}
	writer.addByte(0)
}

func (writer *canonicalWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.buffer.Write(encoded[:])
}

func (writer *canonicalWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *canonicalWriter) addBytes(value []byte) {
	writer.addUint64(uint64(len(value)))
	writer.buffer.Write(value)
}

func (writer canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type canonicalReader struct {
	data   []byte
	offset int
}

func newCanonicalReader(data []byte, domain string) (*canonicalReader, error) {
	if len(data) > maximumCanonicalBytes {
		return nil, fmt.Errorf("canonical payload exceeds %d bytes", maximumCanonicalBytes)
	}
	// The top-level codec owns its input for the duration of decoding. Nested
	// readers therefore keep bounded read-only views instead of copying every
	// framed sub-payload and turning recursive values into quadratic work.
	reader := &canonicalReader{data: data}
	decodedDomain, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode canonical domain: %w", err)
	}
	if decodedDomain != canonicalDomain {
		return nil, fmt.Errorf("unexpected canonical domain %q", decodedDomain)
	}
	decodedType, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode canonical type: %w", err)
	}
	if decodedType != domain {
		return nil, fmt.Errorf("unexpected canonical type %q", decodedType)
	}
	return reader, nil
}

func (reader *canonicalReader) readByte() (byte, error) {
	if reader == nil || reader.offset >= len(reader.data) {
		return 0, fmt.Errorf("unexpected end of canonical payload")
	}
	value := reader.data[reader.offset]
	reader.offset++
	return value, nil
}

func (reader *canonicalReader) readBool() (bool, error) {
	value, err := reader.readByte()
	if err != nil {
		return false, err
	}
	if value == 0 {
		return false, nil
	}
	if value == 1 {
		return true, nil
	}
	return false, fmt.Errorf("non-canonical boolean tag %d", value)
}

func (reader *canonicalReader) readUint64() (uint64, error) {
	if reader == nil || len(reader.data)-reader.offset < 8 {
		return 0, fmt.Errorf("unexpected end of canonical uint64")
	}
	end := reader.offset + 8
	value := binary.BigEndian.Uint64(reader.data[reader.offset:end])
	reader.offset = end
	return value, nil
}

func (reader *canonicalReader) readBytes() ([]byte, error) {
	length, err := reader.readUint64()
	if err != nil {
		return nil, err
	}
	remaining := len(reader.data) - reader.offset
	//nolint:gosec // remaining is non-negative after readUint64 validates the reader bounds.
	if length > uint64(remaining) {
		return nil, fmt.Errorf("canonical byte length %d exceeds remaining payload %d", length, remaining)
	}
	if length > maximumCanonicalBytes {
		return nil, fmt.Errorf("canonical byte field exceeds %d bytes", maximumCanonicalBytes)
	}
	boundedLength := int(length)
	end := reader.offset + boundedLength
	value := reader.data[reader.offset:end]
	reader.offset = end
	return value, nil
}

func (reader *canonicalReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (reader *canonicalReader) readCount() (int, error) {
	count, err := reader.readUint64()
	if err != nil {
		return 0, err
	}
	if count > maximumCollectionSize {
		return 0, fmt.Errorf("canonical collection exceeds %d entries", maximumCollectionSize)
	}
	return int(count), nil
}

func (reader *canonicalReader) readCountLimit(limit int, label string) (int, error) {
	count, err := reader.readCount()
	if err != nil {
		return 0, err
	}
	if count > limit {
		return 0, fmt.Errorf("%s exceeds %d entries", label, limit)
	}
	return count, nil
}

func (reader *canonicalReader) requireDone() error {
	if reader == nil {
		return fmt.Errorf("canonical reader is required")
	}
	if reader.offset != len(reader.data) {
		return fmt.Errorf("canonical payload has %d trailing bytes", len(reader.data)-reader.offset)
	}
	return nil
}

func digestCanonicalBytes(value []byte) string {
	digest := sha256.Sum256(value)
	hexDigest := hex.EncodeToString(digest[:])
	return "sha256:" + hexDigest
}

func equalCanonical(left, right []byte) bool {
	return bytes.Equal(left, right)
}
