package fastxml

import (
	"bytes"
	"errors"
	"unicode/utf8"
)

var (
	cdataPrefByte = []byte(cdataPref)
	cdataSufByte  = []byte(cdataSuf)
)

const (
	cdataPref    = "<![CDATA["
	cdataPrefLen = len(cdataPref)
	cdataSuf     = "]]>"
	cdataSufLen  = len(cdataSuf)
)

// FetchNextToken will return next tag bytes.
//
// Next call to this method must be advanced by the length of the previously returned bytes.
func FetchNextToken(buf []byte) (data []byte, err error) {
	if len(buf) == 0 {
		return nil, nil
	}

	// tagEnd specifies index of end of the tag.
	// Value of 0 tells that not enough data was fed to fetch full tag.
	var tagEnd int

	nextByte := buf[0]

	switch {
	case nextByte == '<': // All XML tags start with '<'.
		tagEnd, err = scanFullTag(buf)
	default: // Treat as text.
		tagEnd, err = scanFullCharData(buf)
	}

	if err != nil {
		return nil, err
	}

	if tagEnd <= 0 {
		return nil, nil
	}

	return buf[:tagEnd], nil
}

// scanFullTag will return end index of the current tag.
//
// It might return error on some broken tags.
func scanFullTag(buf []byte) (int, error) { //nolint:unparam // Error is required to match signature.
	// CDATA needs special treatment as it may contain '>' and '<', and other characters which
	// is forbidden in other tags.
	if bytes.HasPrefix(buf, cdataPrefByte) {
		endIdx := bytes.Index(buf, cdataSufByte)
		if endIdx == -1 {
			return 0, errors.New("no CDATA suffix found")
		}

		return endIdx + cdataSufLen, nil
	}

	return nextTokenStartIndex(buf, '>') + 1, nil
}

// scanFulLCharData will return end index of char data.
func scanFullCharData(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	// We "don't need" to decode every single rune because we will pass this buffer directly as a string.
	// Also as we don't validate XML - no need to be strict about it.
	openIdx := bytes.IndexByte(buf, '<')
	if openIdx == -1 {
		// If no opening char is found - seems that we found the end of the stream.
		// FIXME: Check out which characters are allowed to be added at the end of the file.
		// Some validators say that new line is okay.
		//
		// For now we will assume that all of them are okay(even invalid ones).
		return len(buf), nil
	}

	if !utf8.Valid(buf[:openIdx]) {
		return len(buf), errors.New("invalid utf-8 byte sequence is passed as char data")
	}

	return openIdx, nil
}

// scanTillWordEnd will return index on which valid XML token name will end.
func scanTillWordEnd(buf []byte) int {
	if len(buf) == 0 {
		return 0
	}

	if !isNameStartChar(rune(buf[0])) {
		return 0
	}

	for i := range buf[1:] {
		realIndex := i + 1
		if !isNameChar(rune(buf[realIndex])) {
			return realIndex
		}
	}

	return 1
}

// nextTokenStartIndex checks that in current buffer there is always visible start of next tag.
//
// Function will check range buf[1:] to skip first byte, which can(and will be) be a tag start token.
//
// Value of searchByte must be '<' or '>' or other escapable XML character.
func nextTokenStartIndex(buf []byte, searchByte byte) int {
	if len(buf) < 1 {
		return -1
	}

	idx := bytes.IndexByte(buf[1:], searchByte)
	if idx == -1 {
		return -1
	}

	return idx + 1
}
