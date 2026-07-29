package projectprofile

// These small helpers keep collection transformations linear and make every
// conversion step explicit without spreading imperative loops through the
// domain algebra.
func mapSliceV1Pure[Input any, Output any](
	values []Input,
	convert func(Input) Output,
) []Output {
	size := len(values)
	result := make([]Output, size)
	mapSliceV1PureRange(values, result, convert, 0, size)
	return result
}

func mapSliceV1PureRange[Input any, Output any](
	values []Input,
	result []Output,
	convert func(Input) Output,
	from int,
	until int,
) {
	if from == until {
		return
	}
	if until-from == 1 {
		result[from] = convert(values[from])
		return
	}
	middle := from + (until-from)/2
	mapSliceV1PureRange(values, result, convert, from, middle)
	mapSliceV1PureRange(values, result, convert, middle, until)
}

func mapSliceV1[Input any, Output any](
	values []Input,
	convert func(int, Input) (Output, error),
) ([]Output, error) {
	size := len(values)
	result := make([]Output, size)
	err := mapSliceV1Range(values, result, convert, 0, size)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// mapSliceV1Range divides the collection before descending. Stack depth is
// logarithmic even when an effect-boundary decoder receives a large array.
func mapSliceV1Range[Input any, Output any](
	values []Input,
	result []Output,
	convert func(int, Input) (Output, error),
	from int,
	until int,
) error {
	if from >= until {
		return nil
	}
	if until-from == 1 {
		converted, err := convert(from, values[from])
		if err != nil {
			return err
		}
		result[from] = converted
		return nil
	}
	middle := from + (until-from)/2
	err := mapSliceV1Range(values, result, convert, from, middle)
	if err != nil {
		return err
	}
	return mapSliceV1Range(values, result, convert, middle, until)
}

func visitSliceV1[Value any](
	values []Value,
	visit func(int, Value) error,
) error {
	until := len(values)
	return visitSliceV1Range(values, visit, 0, until)
}

func visitSliceV1Pure[Value any](
	values []Value,
	visit func(Value),
) {
	until := len(values)
	visitSliceV1PureRange(values, visit, 0, until)
}

func visitSliceV1PureRange[Value any](
	values []Value,
	visit func(Value),
	from int,
	until int,
) {
	if from == until {
		return
	}
	if until-from == 1 {
		visit(values[from])
		return
	}
	middle := from + (until-from)/2
	visitSliceV1PureRange(values, visit, from, middle)
	visitSliceV1PureRange(values, visit, middle, until)
}

func visitSliceV1Range[Value any](
	values []Value,
	visit func(int, Value) error,
	from int,
	until int,
) error {
	if from == until {
		return nil
	}
	if until-from == 1 {
		return visit(from, values[from])
	}
	middle := from + (until-from)/2
	err := visitSliceV1Range(values, visit, from, middle)
	if err != nil {
		return err
	}
	return visitSliceV1Range(values, visit, middle, until)
}

func visitAdjacentV1[Value any](
	values []Value,
	visit func(Value, Value) error,
) error {
	until := len(values)
	return visitAdjacentV1Range(values, visit, 1, until)
}

func visitAdjacentV1Range[Value any](
	values []Value,
	visit func(Value, Value) error,
	from int,
	until int,
) error {
	if from >= until {
		return nil
	}
	if until-from == 1 {
		return visit(values[from-1], values[from])
	}
	middle := from + (until-from)/2
	err := visitAdjacentV1Range(values, visit, from, middle)
	if err != nil {
		return err
	}
	return visitAdjacentV1Range(values, visit, middle, until)
}
