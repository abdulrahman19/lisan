package lisan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// entryKeyID is the one key every translation entry must carry.
const entryKeyID = "id"

// entryKeyText holds the string of a non-plural entry.
const entryKeyText = "txt"

// textPosition is a 1-based location within a source file.
type textPosition struct {
	line   int
	column int
}

// sourceFile keeps a decoded file's bytes alongside the line index needed to
// turn a byte offset into a human-readable position.
type sourceFile struct {
	path       string
	data       []byte
	lineStarts []int
}

// newSourceFile indexes the line starts of a file's contents.
func newSourceFile(path string, data []byte) *sourceFile {
	starts := make([]int, 1, bytes.Count(data, []byte{'\n'})+1)

	for index, char := range data {
		if char == '\n' {
			starts = append(starts, index+1)
		}
	}

	return &sourceFile{path: path, data: data, lineStarts: starts}
}

// position converts a byte offset into a 1-based line and column, skipping the
// separators that sit between the offset and the value it refers to.
func (f *sourceFile) position(offset int) textPosition {
	offset = f.skipSeparators(offset)

	index, exact := slices.BinarySearch(f.lineStarts, offset)
	if !exact {
		index--
	}

	if index < 0 {
		index = 0
	}

	return textPosition{line: index + 1, column: offset - f.lineStarts[index] + 1}
}

// skipSeparators advances past whitespace and commas, so that an offset
// reported between two JSON values lands on the start of the next one.
func (f *sourceFile) skipSeparators(offset int) int {
	if offset < 0 {
		offset = 0
	}

	for offset < len(f.data) && isSeparator(f.data[offset]) {
		offset++
	}

	return offset
}

// rawEntry is one decoded translation entry, with the offset it started at.
type rawEntry struct {
	fields map[string]string
	offset int
}

// readEntries decodes a translation file: a JSON array of objects whose values
// are all strings. Offsets are retained so defects can be reported by line.
func readEntries(data []byte) ([]rawEntry, int, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))

	opening, err := decoder.Token()
	if err != nil {
		return nil, offsetOf(err), fmt.Errorf("is not valid JSON: %w", err)
	}

	if delim, isDelim := opening.(json.Delim); !isDelim || delim != '[' {
		return nil, 0, errors.New("must contain a JSON array of translation entries")
	}

	var entries []rawEntry

	for decoder.More() {
		offset := int(decoder.InputOffset())

		fields := make(map[string]string)
		if err := decoder.Decode(&fields); err != nil {
			return nil, offsetOf(err), fmt.Errorf("has an entry that is not an object of strings: %w", err)
		}

		entries = append(entries, rawEntry{fields: fields, offset: offset})
	}

	return entries, 0, nil
}

// offsetOf extracts the byte offset a JSON decoding error refers to.
func offsetOf(err error) int {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return int(syntaxErr.Offset)
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return int(typeErr.Offset)
	}

	return 0
}

// isSeparator reports whether a byte is JSON whitespace or a value separator.
func isSeparator(char byte) bool {
	switch char {
	case ' ', '\t', '\r', '\n', ',':
		return true
	default:
		return false
	}
}
