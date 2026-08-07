package typedmemory

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

const canonicalEnvelopeDomain = "haft.typedmemory.canonical-envelope.v1"

type canonicalWriter struct {
	buffer bytes.Buffer
}

func newCanonicalWriter(domain string) canonicalWriter {
	writer := canonicalWriter{}
	writer.addString(canonicalEnvelopeDomain)
	writer.addString(domain)
	return writer
}

func (writer *canonicalWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *canonicalWriter) addBytes(value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	writer.buffer.Write(length[:])
	writer.buffer.Write(value)
}

func (writer *canonicalWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.addBytes(encoded[:])
}

func (writer canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

func (writer canonicalWriter) digest() SHA256Digest {
	return digestCanonicalBytes(writer.buffer.Bytes())
}

func digestCanonicalBytes(value []byte) SHA256Digest {
	sum := sha256.Sum256(value)
	encoded := hex.EncodeToString(sum[:])
	return SHA256Digest{value: sha256Prefix + encoded}
}

func sortedCanonicalBytes(values [][]byte) [][]byte {
	result := make([][]byte, len(values))
	for index, value := range values {
		result[index] = append([]byte(nil), value...)
	}
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(result[left], result[right]) < 0
	})
	return result
}

func exactTupleKey(domain string, values ...string) string {
	builder := strings.Builder{}
	builder.WriteString(domain)
	for _, value := range values {
		builder.WriteByte(':')
		builder.WriteString(strconv.Itoa(len(value)))
		builder.WriteByte(':')
		builder.WriteString(value)
	}
	return builder.String()
}
