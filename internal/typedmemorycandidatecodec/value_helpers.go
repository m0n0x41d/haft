package typedmemorycandidatecodec

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type typedField struct {
	name  string
	value typedmemory.TypedValue
}

func newTypedRecord(fields []typedField) (typedmemory.TypedValue, error) {
	members := make([]typedmemory.RecordFieldValue, 0, len(fields))
	for _, field := range fields {
		name, err := typedmemory.NewValueMemberName(field.name)
		if err != nil {
			return nil, err
		}
		member, err := typedmemory.NewRecordFieldValue(name, field.value)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	value, err := typedmemory.NewRecordValue(members)
	if err != nil {
		return nil, fmt.Errorf("build typed record: %w", err)
	}
	return value, nil
}

func newTypedSum(
	variant string,
	value typedmemory.TypedValue,
) (typedmemory.TypedValue, error) {
	name, err := typedmemory.NewValueMemberName(variant)
	if err != nil {
		return nil, err
	}
	result, err := typedmemory.NewSumValue(name, value)
	if err != nil {
		return nil, fmt.Errorf("build typed sum: %w", err)
	}
	return result, nil
}
