package typedmemorycandidatecodec

func encodeRawInstantWireForTest(raw string) []byte {
	writer := newCanonicalWriter(canonicalInstantCodecDomain)
	writer = writer.addString(raw)
	return writer.result()
}
