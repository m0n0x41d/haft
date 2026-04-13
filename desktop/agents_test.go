package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTaskOutputBufferKeepsNewestLongSingleLine(t *testing.T) {
	buffer := newTaskOutputBuffer(taskOutputMaxLines, "")
	head := "STARTMARKER"
	tail := strings.Repeat("tail", 2000) + "ENDMARKER"
	body := strings.Repeat("H", taskOutputMaxChars)
	longLine := head + body + tail

	got := buffer.Append(longLine)

	if utf8.RuneCountInString(got) > taskOutputMaxChars {
		t.Fatalf("expected output <= %d runes, got %d", taskOutputMaxChars, utf8.RuneCountInString(got))
	}

	if strings.Contains(got, "STARTMARKER") {
		t.Fatalf("expected oldest prefix marker to be trimmed from output")
	}

	if !strings.HasSuffix(got, "ENDMARKER") {
		t.Fatalf("expected newest output tail to be preserved, got suffix %q", got[maxInt(len(got)-32, 0):])
	}
}

func TestNormalizeTaskOutputKeepsNewestLines(t *testing.T) {
	lines := make([]string, 0, taskOutputMaxLines+25)

	for i := range taskOutputMaxLines + 25 {
		lines = append(lines, fmt.Sprintf("line-%03d", i))
	}

	output := strings.Join(lines, "\n")
	got := normalizeTaskOutput(output)
	gotLines := strings.Split(got, "\n")

	if len(gotLines) != taskOutputMaxLines {
		t.Fatalf("expected %d lines after normalization, got %d", taskOutputMaxLines, len(gotLines))
	}

	if gotLines[0] != "line-025" {
		t.Fatalf("expected first retained line line-025, got %q", gotLines[0])
	}

	if gotLines[len(gotLines)-1] != "line-524" {
		t.Fatalf("expected last retained line line-524, got %q", gotLines[len(gotLines)-1])
	}
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}

	return b
}
