package projecttypeenvactivation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const maximumCanonicalRecordBytes = 32 << 20

type canonicalWriter struct {
	value []byte
}

func newCanonicalWriter(domain string) canonicalWriter {
	writer := canonicalWriter{}
	writer.writeString(domain)
	return writer
}

func (writer *canonicalWriter) writeString(value string) {
	writer.writeBytes([]byte(value))
}

func (writer *canonicalWriter) writeBytes(value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	writer.value = append(writer.value, length[:]...)
	writer.value = append(writer.value, value...)
}

func (writer *canonicalWriter) writeUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.value = append(writer.value, encoded[:]...)
}

func (writer *canonicalWriter) writeUint32(value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	writer.value = append(writer.value, encoded[:]...)
}

func (writer canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.value...)
}

type canonicalReader struct {
	value  []byte
	offset int
}

func newCanonicalReader(
	canonical []byte,
	expectedDomain string,
) (*canonicalReader, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("canonical record is empty")
	}
	if len(canonical) > maximumCanonicalRecordBytes {
		return nil, fmt.Errorf(
			"canonical record exceeds %d bytes",
			maximumCanonicalRecordBytes,
		)
	}
	reader := &canonicalReader{value: canonical}
	domain, err := reader.readString("canonical domain")
	if err != nil {
		return nil, err
	}
	if domain != expectedDomain {
		return nil, fmt.Errorf("canonical domain is invalid")
	}
	return reader, nil
}

func (reader *canonicalReader) readString(name string) (string, error) {
	value, err := reader.readBytes(name)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (reader *canonicalReader) readBytes(name string) ([]byte, error) {
	if reader.remaining() < 8 {
		return nil, fmt.Errorf("%s length is truncated", name)
	}
	length := binary.BigEndian.Uint64(reader.value[reader.offset : reader.offset+8])
	reader.offset += 8
	if length > maximumCanonicalRecordBytes {
		return nil, fmt.Errorf("%s length exceeds the canonical record limit", name)
	}
	boundedLength := int(length)
	if boundedLength > reader.remaining() {
		return nil, fmt.Errorf("%s bytes are truncated", name)
	}
	start := reader.offset
	reader.offset += boundedLength
	return append([]byte(nil), reader.value[start:reader.offset]...), nil
}

func (reader *canonicalReader) readUint64(name string) (uint64, error) {
	if reader.remaining() < 8 {
		return 0, fmt.Errorf("%s is truncated", name)
	}
	value := binary.BigEndian.Uint64(reader.value[reader.offset : reader.offset+8])
	reader.offset += 8
	return value, nil
}

func (reader *canonicalReader) readUint32(name string) (uint32, error) {
	if reader.remaining() < 4 {
		return 0, fmt.Errorf("%s is truncated", name)
	}
	value := binary.BigEndian.Uint32(reader.value[reader.offset : reader.offset+4])
	reader.offset += 4
	return value, nil
}

func (reader *canonicalReader) remaining() int {
	return len(reader.value) - reader.offset
}

func (reader *canonicalReader) requireEnd(name string) error {
	if reader.remaining() != 0 {
		return fmt.Errorf("%s has trailing bytes", name)
	}
	return nil
}

func digestCanonical(domain string, canonical []byte) (string, error) {
	writer := newCanonicalWriter("haft.project-typeenv.selection-effect.digest.v1")
	writer.writeString(domain)
	writer.writeBytes(canonical)
	sum := sha256.Sum256(writer.bytes())
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
