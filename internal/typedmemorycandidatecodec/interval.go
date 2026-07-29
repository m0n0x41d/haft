package typedmemorycandidatecodec

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	performedIntervalCodecDomain          = "haft.local-practice.typed-memory.candidate-codec.performed-interval.v1"
	completedPerformedIntervalCodecDomain = "haft.local-practice.typed-memory.candidate-codec.performed-interval.completed.v1"
	inFlightPerformedIntervalCodecDomain  = "haft.local-practice.typed-memory.candidate-codec.performed-interval.in-flight.v1"
	completedPerformedIntervalVariant     = "Completed"
	inFlightPerformedIntervalVariant      = "InFlight"
)

// PerformedInterval is the closed Haft-local candidate sum. Its name records
// an occurrence interval but does not prove that a record is performed U.Work.
type PerformedInterval interface {
	performedIntervalVariant()
	valid() bool
	variantName() string
	canonicalPayload() []byte
	typedState(CanonicalInstantV1) (typedmemory.TypedValue, error)
}

type CompletedPerformedInterval struct {
	start CanonicalInstant
	end   CanonicalInstant
}

func NewCompletedPerformedInterval(
	start CanonicalInstant,
	end CanonicalInstant,
) (CompletedPerformedInterval, error) {
	if !start.valid() || !end.valid() {
		return CompletedPerformedInterval{}, fmt.Errorf("completed interval requires two canonical instants")
	}
	if end.utc.Before(start.utc) {
		return CompletedPerformedInterval{}, fmt.Errorf("completed interval start must be less than or equal to end")
	}
	return CompletedPerformedInterval{start: start, end: end}, nil
}

func (interval CompletedPerformedInterval) Start() CanonicalInstant {
	return interval.start
}

func (interval CompletedPerformedInterval) End() CanonicalInstant {
	return interval.end
}

func (CompletedPerformedInterval) performedIntervalVariant() {}

func (interval CompletedPerformedInterval) valid() bool {
	return interval.start.valid() &&
		interval.end.valid() &&
		!interval.end.utc.Before(interval.start.utc)
}

func (CompletedPerformedInterval) variantName() string {
	return completedPerformedIntervalVariant
}

func (interval CompletedPerformedInterval) canonicalPayload() []byte {
	start := encodeCanonicalInstantWire(interval.start)
	end := encodeCanonicalInstantWire(interval.end)
	writer := newCanonicalWriter(completedPerformedIntervalCodecDomain)
	writer = writer.addBytes(start)
	writer = writer.addBytes(end)
	return writer.result()
}

func (interval CompletedPerformedInterval) typedState(
	instantCodec CanonicalInstantV1,
) (typedmemory.TypedValue, error) {
	startResult := instantCodec.EncodeInput(interval.start.String())
	start, ok := startResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return nil, rejectionError(startResult)
	}
	endResult := instantCodec.EncodeInput(interval.end.String())
	end, ok := endResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return nil, rejectionError(endResult)
	}
	record, err := newTypedRecord([]typedField{
		{name: "start", value: start.Value()},
		{name: "end", value: end.Value()},
	})
	if err != nil {
		return nil, err
	}
	return newTypedSum(completedPerformedIntervalVariant, record)
}

type InFlightPerformedInterval struct {
	start CanonicalInstant
}

func NewInFlightPerformedInterval(
	start CanonicalInstant,
) (InFlightPerformedInterval, error) {
	if !start.valid() {
		return InFlightPerformedInterval{}, fmt.Errorf("in-flight interval requires one canonical start instant")
	}
	return InFlightPerformedInterval{start: start}, nil
}

func (interval InFlightPerformedInterval) Start() CanonicalInstant {
	return interval.start
}

func (InFlightPerformedInterval) performedIntervalVariant() {}

func (interval InFlightPerformedInterval) valid() bool {
	return interval.start.valid()
}

func (InFlightPerformedInterval) variantName() string {
	return inFlightPerformedIntervalVariant
}

func (interval InFlightPerformedInterval) canonicalPayload() []byte {
	start := encodeCanonicalInstantWire(interval.start)
	writer := newCanonicalWriter(inFlightPerformedIntervalCodecDomain)
	writer = writer.addBytes(start)
	return writer.result()
}

func (interval InFlightPerformedInterval) typedState(
	instantCodec CanonicalInstantV1,
) (typedmemory.TypedValue, error) {
	startResult := instantCodec.EncodeInput(interval.start.String())
	start, ok := startResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return nil, rejectionError(startResult)
	}
	record, err := newTypedRecord([]typedField{
		{name: "start", value: start.Value()},
	})
	if err != nil {
		return nil, err
	}
	return newTypedSum(inFlightPerformedIntervalVariant, record)
}

// PerformedIntervalV1 implements the candidate sum and interval-order rule.
type PerformedIntervalV1 struct {
	shape   typedmemory.ValueShapeRef
	instant CanonicalInstantV1
}

func (codec PerformedIntervalV1) Shape() typedmemory.ValueShapeRef {
	return codec.shape
}

func (codec PerformedIntervalV1) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	inputBytes []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return rejectShape("PerformedIntervalV1", codec.shape, expectedShape)
	}
	value, err := decodePerformedIntervalWire(inputBytes)
	if err != nil {
		return rejectMalformed(
			"PerformedIntervalV1",
			"typed_value.performed_interval",
			err,
		)
	}
	return codec.canonicalizeValue(value)
}

func (codec PerformedIntervalV1) EncodeInput(
	value PerformedInterval,
) typedmemory.CodecCanonicalization {
	if value == nil || !value.valid() {
		err := fmt.Errorf("performed interval is outside the closed valid sum")
		return rejectMalformed(
			"PerformedIntervalV1",
			"typed_value.performed_interval",
			err,
		)
	}
	return codec.canonicalizeValue(value)
}

func (codec PerformedIntervalV1) canonicalizeValue(
	value PerformedInterval,
) typedmemory.CodecCanonicalization {
	state, err := value.typedState(codec.instant)
	if err != nil {
		return rejectMalformed(
			"PerformedIntervalV1",
			"typed_value.performed_interval.state",
			err,
		)
	}
	typed, err := newTypedRecord([]typedField{
		{name: "state", value: state},
	})
	if err != nil {
		return rejectMalformed(
			"PerformedIntervalV1",
			"typed_value.performed_interval",
			err,
		)
	}
	canonical := encodePerformedIntervalWire(value)
	return acceptCanonical("PerformedIntervalV1", typed, canonical)
}

func encodePerformedIntervalWire(value PerformedInterval) []byte {
	writer := newCanonicalWriter(performedIntervalCodecDomain)
	writer = writer.addString(value.variantName())
	writer = writer.addBytes(value.canonicalPayload())
	return writer.result()
}

func decodePerformedIntervalWire(input []byte) (PerformedInterval, error) {
	reader, err := newCanonicalReader(input, performedIntervalCodecDomain)
	if err != nil {
		return nil, err
	}
	variant, reader, err := reader.readString()
	if err != nil {
		return nil, err
	}
	payload, reader, err := reader.readBytes()
	if err != nil {
		return nil, err
	}
	if err := reader.requireEnd(); err != nil {
		return nil, err
	}
	decoders := map[string]func([]byte) (PerformedInterval, error){
		completedPerformedIntervalVariant: decodeCompletedPerformedInterval,
		inFlightPerformedIntervalVariant:  decodeInFlightPerformedInterval,
	}
	decode, exists := decoders[variant]
	if !exists {
		return nil, fmt.Errorf("performed interval variant %q is unknown", variant)
	}
	return decode(payload)
}

func decodeCompletedPerformedInterval(
	payload []byte,
) (PerformedInterval, error) {
	reader, err := newCanonicalReader(payload, completedPerformedIntervalCodecDomain)
	if err != nil {
		return nil, err
	}
	startBytes, reader, err := reader.readBytes()
	if err != nil {
		return nil, err
	}
	endBytes, reader, err := reader.readBytes()
	if err != nil {
		return nil, err
	}
	if err := reader.requireEnd(); err != nil {
		return nil, err
	}
	start, err := decodeCanonicalInstantWire(startBytes)
	if err != nil {
		return nil, fmt.Errorf("completed start: %w", err)
	}
	end, err := decodeCanonicalInstantWire(endBytes)
	if err != nil {
		return nil, fmt.Errorf("completed end: %w", err)
	}
	return NewCompletedPerformedInterval(start, end)
}

func decodeInFlightPerformedInterval(
	payload []byte,
) (PerformedInterval, error) {
	reader, err := newCanonicalReader(payload, inFlightPerformedIntervalCodecDomain)
	if err != nil {
		return nil, err
	}
	startBytes, reader, err := reader.readBytes()
	if err != nil {
		return nil, err
	}
	if err := reader.requireEnd(); err != nil {
		return nil, err
	}
	start, err := decodeCanonicalInstantWire(startBytes)
	if err != nil {
		return nil, fmt.Errorf("in-flight start: %w", err)
	}
	return NewInFlightPerformedInterval(start)
}

var _ typedmemory.CodecImplementation = PerformedIntervalV1{}
