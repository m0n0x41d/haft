package typedmemorywire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const (
	MaximumRequestBytes = 512 * 1024
	MaximumJSONDepth    = 32
	MaximumArrayItems   = 64
	MaximumObjectFields = 64
	maximumJSONTokens   = 64 * 1024
	maximumScalarBytes  = 384 * 1024
)

type ErrorCode string

const (
	ErrorMalformedJSON   ErrorCode = "malformed_json"
	ErrorResourceLimit   ErrorCode = "resource_limit"
	ErrorInvalidContract ErrorCode = "invalid_contract"
)

type DecodeError struct {
	code    ErrorCode
	path    string
	message string
}

func (decodeError *DecodeError) Error() string {
	return fmt.Sprintf("%s at %s: %s", decodeError.code, decodeError.path, decodeError.message)
}

func (decodeError *DecodeError) Code() ErrorCode { return decodeError.code }

func (decodeError *DecodeError) Path() string { return decodeError.path }

func (decodeError *DecodeError) Message() string { return decodeError.message }

func malformedJSON(path, message string) error {
	return &DecodeError{code: ErrorMalformedJSON, path: path, message: message}
}

func resourceLimit(path, message string) error {
	return &DecodeError{code: ErrorResourceLimit, path: path, message: message}
}

func invalidContract(path, message string) error {
	return &DecodeError{code: ErrorInvalidContract, path: path, message: message}
}

type scanFrame struct {
	kind         json.Delim
	path         string
	seen         map[string]struct{}
	expectingKey bool
	pendingKey   string
	count        int
}

func scanStrictJSON(payload []byte) error {
	if len(payload) == 0 {
		return malformedJSON("$", "request body is empty")
	}
	if len(payload) > MaximumRequestBytes {
		message := fmt.Sprintf("request exceeds %d bytes", MaximumRequestBytes)
		return resourceLimit("$", message)
	}

	reader := bytes.NewReader(payload)
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	frames := make([]scanFrame, 0, MaximumJSONDepth)
	rootConsumed := false
	tokenCount := 0

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			path := currentScanPath(frames)
			return malformedJSON(path, err.Error())
		}

		tokenCount++
		if tokenCount > maximumJSONTokens {
			path := currentScanPath(frames)
			message := fmt.Sprintf("JSON exceeds %d tokens", maximumJSONTokens)
			return resourceLimit(path, message)
		}

		delimiter, isDelimiter := token.(json.Delim)
		if isDelimiter {
			updatedFrames, updatedRoot, delimiterErr := scanDelimiter(frames, rootConsumed, delimiter)
			if delimiterErr != nil {
				return delimiterErr
			}
			frames = updatedFrames
			rootConsumed = updatedRoot
			continue
		}

		if token == nil {
			path, _, pathErr := beginScanValue(frames, rootConsumed)
			if pathErr != nil {
				return pathErr
			}
			return invalidContract(path, "null is not allowed")
		}

		text, isString := token.(string)
		if isString && expectingObjectKey(frames) {
			updatedFrames, keyErr := scanObjectKey(frames, text)
			if keyErr != nil {
				return keyErr
			}
			frames = updatedFrames
			continue
		}

		path, updatedRoot, valueErr := beginScanValue(frames, rootConsumed)
		if valueErr != nil {
			return valueErr
		}
		rootConsumed = updatedRoot
		if isString && len(text) > maximumScalarBytes {
			message := fmt.Sprintf("string exceeds %d bytes", maximumScalarBytes)
			return resourceLimit(path, message)
		}
		frames = completeScanValue(frames)
	}

	if len(frames) != 0 {
		path := currentScanPath(frames)
		return malformedJSON(path, "JSON container is not closed")
	}
	if !rootConsumed {
		return malformedJSON("$", "request body has no JSON value")
	}
	return nil
}

func scanDelimiter(
	frames []scanFrame,
	rootConsumed bool,
	delimiter json.Delim,
) ([]scanFrame, bool, error) {
	switch delimiter {
	case '{', '[':
		path, updatedRoot, err := beginScanValue(frames, rootConsumed)
		if err != nil {
			return frames, rootConsumed, err
		}
		if len(frames)+1 > MaximumJSONDepth {
			message := fmt.Sprintf("JSON nesting exceeds depth %d", MaximumJSONDepth)
			return frames, rootConsumed, resourceLimit(path, message)
		}

		frame := scanFrame{kind: delimiter, path: path}
		if delimiter == '{' {
			frame.seen = make(map[string]struct{})
			frame.expectingKey = true
		}
		return append(frames, frame), updatedRoot, nil
	case '}', ']':
		if len(frames) == 0 {
			return frames, rootConsumed, malformedJSON("$", "unexpected closing delimiter")
		}
		top := frames[len(frames)-1]
		expected := matchingClose(top.kind)
		if delimiter != expected {
			message := fmt.Sprintf("unexpected %q; expected %q", delimiter, expected)
			return frames, rootConsumed, malformedJSON(top.path, message)
		}
		if top.kind == '{' && !top.expectingKey {
			return frames, rootConsumed, malformedJSON(top.path, "object field has no value")
		}

		frames = frames[:len(frames)-1]
		frames = completeScanValue(frames)
		return frames, rootConsumed, nil
	default:
		path := currentScanPath(frames)
		message := fmt.Sprintf("unsupported delimiter %q", delimiter)
		return frames, rootConsumed, malformedJSON(path, message)
	}
}

func beginScanValue(frames []scanFrame, rootConsumed bool) (string, bool, error) {
	if len(frames) == 0 {
		if rootConsumed {
			return "$", rootConsumed, malformedJSON("$", "trailing JSON value is not allowed")
		}
		return "$", true, nil
	}

	index := len(frames) - 1
	top := &frames[index]
	if top.kind == '{' {
		if top.expectingKey {
			return top.path, rootConsumed, malformedJSON(top.path, "object value requires a field name")
		}
		return top.path + "." + top.pendingKey, rootConsumed, nil
	}

	if top.count >= MaximumArrayItems {
		message := fmt.Sprintf("array exceeds %d items", MaximumArrayItems)
		return top.path, rootConsumed, resourceLimit(top.path, message)
	}
	path := fmt.Sprintf("%s[%d]", top.path, top.count)
	top.count++
	return path, rootConsumed, nil
}

func completeScanValue(frames []scanFrame) []scanFrame {
	if len(frames) == 0 {
		return frames
	}
	index := len(frames) - 1
	if frames[index].kind != '{' {
		return frames
	}
	frames[index].expectingKey = true
	frames[index].pendingKey = ""
	return frames
}

func expectingObjectKey(frames []scanFrame) bool {
	if len(frames) == 0 {
		return false
	}
	top := frames[len(frames)-1]
	return top.kind == '{' && top.expectingKey
}

func scanObjectKey(frames []scanFrame, key string) ([]scanFrame, error) {
	index := len(frames) - 1
	top := &frames[index]
	if len(key) > 4096 {
		return frames, resourceLimit(top.path, "object field name exceeds 4096 bytes")
	}
	if top.count >= MaximumObjectFields {
		message := fmt.Sprintf("object exceeds %d fields", MaximumObjectFields)
		return frames, resourceLimit(top.path, message)
	}
	if _, duplicate := top.seen[key]; duplicate {
		return frames, invalidContract(top.path+"."+key, "duplicate object field")
	}
	top.seen[key] = struct{}{}
	top.count++
	top.expectingKey = false
	top.pendingKey = key
	return frames, nil
}

func currentScanPath(frames []scanFrame) string {
	if len(frames) == 0 {
		return "$"
	}
	return frames[len(frames)-1].path
}

func matchingClose(open json.Delim) json.Delim {
	if open == '{' {
		return '}'
	}
	return ']'
}
