package typedmemorycandidatecodec

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"unicode/utf8"
)

const candidateCodecEnvelopeDomain = "haft.local-practice.typed-memory.candidate-codec-envelope.v1"

type canonicalWriter struct {
	bytes []byte
}

func newCanonicalWriter(domain string) canonicalWriter {
	writer := canonicalWriter{}
	writer = writer.addString(candidateCodecEnvelopeDomain)
	writer = writer.addString(domain)
	return writer
}

func (writer canonicalWriter) addString(value string) canonicalWriter {
	return writer.addBytes([]byte(value))
}

func (writer canonicalWriter) addBytes(value []byte) canonicalWriter {
	length := make([]byte, 8)
	binary.BigEndian.PutUint64(length, uint64(len(value)))
	result := make([]byte, 0, len(writer.bytes)+len(length)+len(value))
	result = append(result, writer.bytes...)
	result = append(result, length...)
	result = append(result, value...)
	return canonicalWriter{bytes: result}
}

func (writer canonicalWriter) result() []byte {
	return append([]byte(nil), writer.bytes...)
}

type canonicalReader struct {
	bytes  []byte
	offset int
}

func newCanonicalReader(input []byte, domain string) (canonicalReader, error) {
	reader := canonicalReader{bytes: append([]byte(nil), input...)}
	envelope, next, err := reader.readString()
	if err != nil {
		return canonicalReader{}, err
	}
	if envelope != candidateCodecEnvelopeDomain {
		return canonicalReader{}, fmt.Errorf("canonical envelope domain is %q", envelope)
	}
	encodedDomain, next, err := next.readString()
	if err != nil {
		return canonicalReader{}, err
	}
	if encodedDomain != domain {
		return canonicalReader{}, fmt.Errorf("canonical value domain is %q", encodedDomain)
	}
	return next, nil
}

func (reader canonicalReader) readString() (string, canonicalReader, error) {
	value, next, err := reader.readBytes()
	if err != nil {
		return "", canonicalReader{}, err
	}
	if !utf8.Valid(value) {
		return "", canonicalReader{}, fmt.Errorf("canonical string is not valid UTF-8")
	}
	return string(value), next, nil
}

func (reader canonicalReader) readBytes() ([]byte, canonicalReader, error) {
	remaining := len(reader.bytes) - reader.offset
	if remaining < 8 {
		return nil, canonicalReader{}, fmt.Errorf("canonical length frame is truncated")
	}
	lengthBytes := reader.bytes[reader.offset : reader.offset+8]
	length := binary.BigEndian.Uint64(lengthBytes)
	payloadOffset := reader.offset + 8
	payloadRemaining := len(reader.bytes) - payloadOffset
	payloadRemainingValue, err := strconv.ParseUint(strconv.Itoa(payloadRemaining), 10, 64)
	if err != nil {
		return nil, canonicalReader{}, fmt.Errorf("canonical remaining byte count is invalid: %w", err)
	}
	if length > payloadRemainingValue {
		return nil, canonicalReader{}, fmt.Errorf("canonical length frame exceeds remaining bytes")
	}
	lengthValue, err := strconv.Atoi(strconv.FormatUint(length, 10))
	if err != nil {
		return nil, canonicalReader{}, fmt.Errorf("canonical length frame does not fit this runtime: %w", err)
	}
	payloadEnd := payloadOffset + lengthValue
	value := append([]byte(nil), reader.bytes[payloadOffset:payloadEnd]...)
	next := canonicalReader{bytes: reader.bytes, offset: payloadEnd}
	return value, next, nil
}

func (reader canonicalReader) requireEnd() error {
	if reader.offset != len(reader.bytes) {
		return fmt.Errorf("canonical value has %d trailing bytes", len(reader.bytes)-reader.offset)
	}
	return nil
}
