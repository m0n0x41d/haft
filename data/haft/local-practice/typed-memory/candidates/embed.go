// Package candidates exposes the exact shipped Haft typed-memory
// Local-Practice carrier to the installed binary. The YAML file beside this
// source remains the single authoritative carrier byte stream.
package candidates

import _ "embed"

const (
	baseTypeEnvRefV1   = "typeenv:sha256:aa1eec077868e611108810f1e4bc187d55eb38e3bc705cc149a098008b58cd1a"
	baseTypeEnvRefV1_1 = "typeenv:sha256:a5223d5018230095652543f0378a1fc3f64175f21d01309e6f4084088d5d2804"
	baseTypeEnvRefV1_2 = "typeenv:sha256:973eeeed8e234b4ff0194662d80e204fe27ad5ba92c87840a6d1ed3a9d5d742d"
	baseTypeEnvRefV1_3 = "typeenv:sha256:28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6"
	baseTypeEnvRefV1_4 = "typeenv:sha256:effff65cae9eaf1aba287245df79c460fbeaee5f666dcaa7992bfeb251c1e35e"
	baseTypeEnvRefV1_5 = "typeenv:sha256:1b6b04c14aa43bea396aafdbd810eb0345f7f9e9be37a5aee874a328c3b26efc"
	baseTypeEnvRefV1_6 = "typeenv:sha256:dffe960ad95df0f16c66c4040dfcb3c20ea19dc1aa1a4d506bb1dae77e514565"
)

// sourceV1 is the exact 1.0.0 Local-Practice publication candidate.
//
//go:embed 1.0.0.yaml
var sourceV1 []byte

// SourceV1 returns a private copy of the exact source carrier bytes.
func SourceV1() []byte {
	return append([]byte(nil), sourceV1...)
}

// sourceV1_1 is the exact 1.1.0 Local-Practice publication candidate. It is a
// successor source stream; sourceV1 remains embedded byte-for-byte for
// historical replay.
//
//go:embed 1.1.0.yaml
var sourceV1_1 []byte

// SourceV1_1 returns a private copy of the exact 1.1.0 source carrier bytes.
func SourceV1_1() []byte {
	return append([]byte(nil), sourceV1_1...)
}

// sourceV1_2 is the exact sealed 1.2.0 Local-Practice publication candidate.
// It remains embedded byte-for-byte with its historical Base TypeEnv for
// exact replay.
//
//go:embed 1.2.0.yaml
var sourceV1_2 []byte

// SourceV1_2 returns a private copy of the exact 1.2.0 source carrier bytes.
func SourceV1_2() []byte {
	return append([]byte(nil), sourceV1_2...)
}

// sourceV1_3 is the first C.3 KindClassification Local-Practice candidate.
// It remains embedded byte-for-byte with its historical Base TypeEnv.
//
//go:embed 1.3.0.yaml
var sourceV1_3 []byte

// SourceV1_3 returns a private copy of the exact 1.3.0 source carrier bytes.
func SourceV1_3() []byte {
	return append([]byte(nil), sourceV1_3...)
}

// sourceV1_4 carries the same declaration set as 1.3.0 at the current exact
// FPF Base and source coordinates. It is a successor source stream; sourceV1_3
// remains embedded byte-for-byte for historical replay.
//
//go:embed 1.4.0.yaml
var sourceV1_4 []byte

// SourceV1_4 returns a private copy of the exact 1.4.0 source carrier bytes.
func SourceV1_4() []byte {
	return append([]byte(nil), sourceV1_4...)
}

// sourceV1_5 carries the 1.4.0 declaration set at the current exact FPF Base
// and source coordinates. It is a successor source stream; sourceV1_4 remains
// embedded byte-for-byte for historical replay.
//
//go:embed 1.5.0.yaml
var sourceV1_5 []byte

// SourceV1_5 returns a private copy of the exact 1.5.0 source carrier bytes.
func SourceV1_5() []byte {
	return append([]byte(nil), sourceV1_5...)
}

// sourceV1_6 carries the 1.5.0 declaration set at the current exact FPF Base
// and source coordinates. It is a successor source stream; sourceV1_5 remains
// embedded byte-for-byte for historical replay.
//
//go:embed 1.6.0.yaml
var sourceV1_6 []byte

// SourceV1_6 returns a private copy of the exact 1.6.0 source carrier bytes.
func SourceV1_6() []byte {
	return append([]byte(nil), sourceV1_6...)
}

// SourcesForExactBaseTypeEnvRef resolves shipped carriers only from the exact
// B coordinate each declares. It keeps the historical 1.0.0 bytes available
// for already-persisted Genesis replay while exposing every successor only at
// its exact base. The caller must still match the reviewed E/X/C coordinates;
// a Base TypeEnv can legitimately have several Local-Practice successors.
func SourcesForExactBaseTypeEnvRef(ref string) [][]byte {
	type sourceCoordinate struct {
		base   string
		source []byte
	}
	candidates := []sourceCoordinate{
		{base: baseTypeEnvRefV1, source: sourceV1},
		{base: baseTypeEnvRefV1_1, source: sourceV1_1},
		{base: baseTypeEnvRefV1_2, source: sourceV1_2},
		{base: baseTypeEnvRefV1_3, source: sourceV1_3},
		{base: baseTypeEnvRefV1_4, source: sourceV1_4},
		{base: baseTypeEnvRefV1_5, source: sourceV1_5},
		{base: baseTypeEnvRefV1_6, source: sourceV1_6},
	}
	result := make([][]byte, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.base != ref {
			continue
		}
		result = append(result, append([]byte(nil), candidate.source...))
	}
	return result
}
