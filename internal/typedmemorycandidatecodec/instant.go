package typedmemorycandidatecodec

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const canonicalInstantCodecDomain = "haft.local-practice.typed-memory.candidate-codec.canonical-instant.v1"

var canonicalInstantInputPattern = regexp.MustCompile(
	`^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$`,
)

// CanonicalInstant is an immutable UTC instant within the candidate's closed
// year range. String returns the unique canonical text ending in Z.
type CanonicalInstant struct {
	utc       time.Time
	canonical string
}

func ParseCanonicalInstant(raw string) (CanonicalInstant, error) {
	parts := canonicalInstantInputPattern.FindStringSubmatch(raw)
	if parts == nil {
		return CanonicalInstant{}, fmt.Errorf("instant does not match the exact candidate grammar")
	}
	year, err := parseInstantNumber("year", parts[1])
	if err != nil {
		return CanonicalInstant{}, err
	}
	month, err := parseInstantNumber("month", parts[2])
	if err != nil {
		return CanonicalInstant{}, err
	}
	day, err := parseInstantNumber("day", parts[3])
	if err != nil {
		return CanonicalInstant{}, err
	}
	hour, err := parseInstantNumber("hour", parts[4])
	if err != nil {
		return CanonicalInstant{}, err
	}
	minute, err := parseInstantNumber("minute", parts[5])
	if err != nil {
		return CanonicalInstant{}, err
	}
	second, err := parseInstantNumber("second", parts[6])
	if err != nil {
		return CanonicalInstant{}, err
	}
	nanosecond, err := parseInstantFraction(parts[7])
	if err != nil {
		return CanonicalInstant{}, err
	}
	offsetSeconds, err := parseInstantOffset(parts[8])
	if err != nil {
		return CanonicalInstant{}, err
	}
	if year < 1 || year > 9999 {
		return CanonicalInstant{}, fmt.Errorf("year must be 0001 through 9999")
	}
	if hour > 23 || minute > 59 || second > 59 {
		return CanonicalInstant{}, fmt.Errorf("time-of-day component is out of range")
	}
	location := time.FixedZone("candidate-offset", offsetSeconds)
	local := time.Date(
		year,
		time.Month(month),
		day,
		hour,
		minute,
		second,
		nanosecond,
		location,
	)
	if !instantComponentsMatch(local, year, month, day, hour, minute, second, nanosecond) {
		return CanonicalInstant{}, fmt.Errorf("calendar date is invalid")
	}
	utc := local.UTC()
	if utc.Year() < 1 || utc.Year() > 9999 {
		return CanonicalInstant{}, fmt.Errorf("UTC normalization leaves years 0001 through 9999")
	}
	canonical := formatCanonicalInstant(utc)
	return CanonicalInstant{utc: utc, canonical: canonical}, nil
}

func (instant CanonicalInstant) String() string { return instant.canonical }

func (instant CanonicalInstant) valid() bool {
	if instant.canonical == "" || instant.utc.Location() != time.UTC {
		return false
	}
	return formatCanonicalInstant(instant.utc) == instant.canonical
}

func parseInstantNumber(label string, raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s is not decimal", label)
	}
	return value, nil
}

func parseInstantFraction(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	padded := raw + strings.Repeat("0", 9-len(raw))
	value, err := strconv.Atoi(padded)
	if err != nil {
		return 0, fmt.Errorf("fraction is not decimal")
	}
	return value, nil
}

func parseInstantOffset(raw string) (int, error) {
	if raw == "Z" {
		return 0, nil
	}
	sign := 1
	if raw[0] == '-' {
		sign = -1
	}
	hour, err := parseInstantNumber("offset hour", raw[1:3])
	if err != nil {
		return 0, err
	}
	minute, err := parseInstantNumber("offset minute", raw[4:6])
	if err != nil {
		return 0, err
	}
	if minute > 59 || hour > 14 || hour == 14 && minute != 0 {
		return 0, fmt.Errorf("offset is beyond plus-or-minus 14:00")
	}
	if sign < 0 && hour == 0 && minute == 0 {
		return 0, fmt.Errorf("-00:00 offset is forbidden")
	}
	seconds := hour*60*60 + minute*60
	return sign * seconds, nil
}

func instantComponentsMatch(
	value time.Time,
	year int,
	month int,
	day int,
	hour int,
	minute int,
	second int,
	nanosecond int,
) bool {
	return value.Year() == year &&
		int(value.Month()) == month &&
		value.Day() == day &&
		value.Hour() == hour &&
		value.Minute() == minute &&
		value.Second() == second &&
		value.Nanosecond() == nanosecond
}

func formatCanonicalInstant(value time.Time) string {
	base := fmt.Sprintf(
		"%04d-%02d-%02dT%02d:%02d:%02d",
		value.Year(),
		value.Month(),
		value.Day(),
		value.Hour(),
		value.Minute(),
		value.Second(),
	)
	if value.Nanosecond() == 0 {
		return base + "Z"
	}
	fraction := fmt.Sprintf("%09d", value.Nanosecond())
	fraction = strings.TrimRight(fraction, "0")
	return base + "." + fraction + "Z"
}

// CanonicalInstantV1 implements the candidate instant grammar and UTC
// canonicalization. It has no chronology or performed-Work semantics.
type CanonicalInstantV1 struct {
	shape typedmemory.ValueShapeRef
}

func (codec CanonicalInstantV1) Shape() typedmemory.ValueShapeRef {
	return codec.shape
}

func (codec CanonicalInstantV1) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	inputBytes []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return rejectShape("CanonicalInstantV1", codec.shape, expectedShape)
	}
	value, err := decodeCanonicalInstantWire(inputBytes)
	if err != nil {
		return rejectMalformed(
			"CanonicalInstantV1",
			"typed_value.canonical_instant",
			err,
		)
	}
	canonical := encodeCanonicalInstantWire(value)
	typed := typedmemory.NewTextValue(value.String())
	return acceptCanonical("CanonicalInstantV1", typed, canonical)
}

func (codec CanonicalInstantV1) EncodeInput(
	raw string,
) typedmemory.CodecCanonicalization {
	value, err := ParseCanonicalInstant(raw)
	if err != nil {
		return rejectMalformed(
			"CanonicalInstantV1",
			"typed_value.canonical_instant",
			err,
		)
	}
	canonical := encodeCanonicalInstantWire(value)
	typed := typedmemory.NewTextValue(value.String())
	return acceptCanonical("CanonicalInstantV1", typed, canonical)
}

func encodeCanonicalInstantWire(value CanonicalInstant) []byte {
	writer := newCanonicalWriter(canonicalInstantCodecDomain)
	writer = writer.addString(value.String())
	return writer.result()
}

func decodeCanonicalInstantWire(input []byte) (CanonicalInstant, error) {
	reader, err := newCanonicalReader(input, canonicalInstantCodecDomain)
	if err != nil {
		return CanonicalInstant{}, err
	}
	raw, reader, err := reader.readString()
	if err != nil {
		return CanonicalInstant{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return CanonicalInstant{}, err
	}
	return ParseCanonicalInstant(raw)
}

var _ typedmemory.CodecImplementation = CanonicalInstantV1{}
