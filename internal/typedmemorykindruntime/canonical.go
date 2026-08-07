package typedmemorykindruntime

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type canonicalWriter struct {
	data []byte
}

func newCanonicalWriter(domain string) canonicalWriter {
	writer := canonicalWriter{}
	writer.addString(domain)
	return writer
}

func (writer *canonicalWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *canonicalWriter) addUint64(value uint64) {
	frame := make([]byte, 8)
	binary.BigEndian.PutUint64(frame, value)
	writer.addBytes(frame)
}

func (writer *canonicalWriter) addBytes(value []byte) {
	length := make([]byte, 8)
	binary.BigEndian.PutUint64(length, uint64(len(value)))
	writer.data = append(writer.data, length...)
	writer.data = append(writer.data, value...)
}

func (writer canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.data...)
}

func (writer canonicalWriter) digest() typedmemory.SHA256Digest {
	sum := sha256.Sum256(writer.data)
	encoded := "sha256:" + hex.EncodeToString(sum[:])
	digest, _ := typedmemory.NewSHA256Digest(encoded)
	return digest
}
