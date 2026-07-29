package projecttypeenvselectioneffect

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	maximumCanonicalRecordBytes = 32 << 20
	maximumCanonicalTextBytes   = 1 << 20
	maximumOrderedExtensions    = 4096
)

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

func (reader *canonicalReader) readBytes(name string) ([]byte, error) {
	length, err := reader.readUint64(name + " length")
	if err != nil {
		return nil, err
	}
	if length > maximumCanonicalRecordBytes {
		return nil, fmt.Errorf("%s exceeds canonical record limit", name)
	}
	byteCount := int(length)
	if byteCount > reader.remaining() {
		return nil, fmt.Errorf("%s is truncated", name)
	}
	start := reader.offset
	reader.offset += byteCount
	return append([]byte(nil), reader.value[start:reader.offset]...), nil
}

func (reader *canonicalReader) readString(name string) (string, error) {
	encoded, err := reader.readBytes(name)
	if err != nil {
		return "", err
	}
	if len(encoded) > maximumCanonicalTextBytes {
		return "", fmt.Errorf("%s exceeds %d bytes", name, maximumCanonicalTextBytes)
	}
	if !utf8.Valid(encoded) {
		return "", fmt.Errorf("%s is not valid UTF-8", name)
	}
	return string(encoded), nil
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

func digestCanonical(
	domain string,
	canonical []byte,
) (typedmemory.SHA256Digest, error) {
	writer := newCanonicalWriter("haft.project-typeenv.selection-effect.digest.v1")
	writer.writeString(domain)
	writer.writeBytes(canonical)
	return digestRaw(writer.bytes())
}

func digestFields(
	domain string,
	fields ...string,
) (typedmemory.SHA256Digest, error) {
	writer := newCanonicalWriter("haft.project-typeenv.selection-effect.fields.v1")
	writer.writeString(domain)
	for _, field := range fields {
		writer.writeString(field)
	}
	return digestRaw(writer.bytes())
}

func digestRaw(value []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(value)
	encoded := hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest("sha256:" + encoded)
}

func canonicalHex(value typedmemory.SHA256Digest) string {
	return strings.TrimPrefix(value.String(), "sha256:")
}

func countToUint32(name string, count int) (uint32, error) {
	if count < 0 || uint64(count) > uint64(math.MaxUint32) {
		return 0, fmt.Errorf("%s count is out of range", name)
	}
	return uint32(count), nil
}
