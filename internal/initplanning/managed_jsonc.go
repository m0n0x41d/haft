package initplanning

import (
	"fmt"
)

func decodeManagedJSON(
	raw []byte,
	mergeEdition string,
) (any, error) {
	if mergeEdition != ManagedJSONCRewriteMergeEdition {
		return decodeUniqueJSON(raw)
	}
	normalized, err := normalizeManagedJSONC(raw)
	if err != nil {
		return nil, err
	}
	return decodeUniqueJSON(normalized)
}

// normalizeManagedJSONC removes JSONC comments and trailing commas without
// interpreting string contents. It is deliberately a normalization step
// before the duplicate-key-aware strict JSON decoder, not a second JSON
// parser.
func normalizeManagedJSONC(raw []byte) ([]byte, error) {
	result := make([]byte, 0, len(raw))
	inString := false
	escaped := false
	for index := 0; index < len(raw); {
		current := raw[index]
		if inString {
			result = append(result, current)
			switch {
			case escaped:
				escaped = false
			case current == '\\':
				escaped = true
			case current == '"':
				inString = false
			}
			index++
			continue
		}
		if current == '"' {
			inString = true
			result = append(result, current)
			index++
			continue
		}
		if current == '/' && index+1 < len(raw) &&
			raw[index+1] == '/' {
			result = append(result, ' ', ' ')
			index += 2
			for index < len(raw) && raw[index] != '\n' {
				result = append(result, ' ')
				index++
			}
			continue
		}
		if current == '/' && index+1 < len(raw) &&
			raw[index+1] == '*' {
			result = append(result, ' ', ' ')
			index += 2
			closed := false
			for index < len(raw) {
				if index+1 < len(raw) &&
					raw[index] == '*' &&
					raw[index+1] == '/' {
					result = append(result, ' ', ' ')
					index += 2
					closed = true
					break
				}
				if raw[index] == '\n' || raw[index] == '\r' {
					result = append(result, raw[index])
				} else {
					result = append(result, ' ')
				}
				index++
			}
			if !closed {
				return nil, fmt.Errorf(
					"managed JSONC carrier has an unterminated block comment",
				)
			}
			continue
		}
		if current == '}' || current == ']' {
			result = blankManagedJSONCTrailingComma(result)
		}
		result = append(result, current)
		index++
	}
	return result, nil
}

func blankManagedJSONCTrailingComma(raw []byte) []byte {
	index := len(raw)
	for index > 0 {
		candidate := raw[index-1]
		if candidate != ' ' &&
			candidate != '\t' &&
			candidate != '\n' &&
			candidate != '\r' {
			break
		}
		index--
	}
	if index > 0 && raw[index-1] == ',' {
		raw[index-1] = ' '
	}
	return raw
}
