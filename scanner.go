package fastxml

import (
	"bytes"
	"errors"
	"fmt"
)

var (
	cdataPrefix   = []byte("<![CDATA[")
	cdataSuffix   = []byte("]]>")
	commentPrefix = []byte("<!--")
	commentSuffix = []byte("-->")
)

var (
	cdataPrefLen = len(cdataPrefix)
	cdataSufLen  = len(cdataSuffix)
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

	switch {
	case isSpecialTag(buf):
		tagEnd, err = scanSpecial(buf)
	case buf[0] == '<': // All XML tags start with '<'.
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

func isSpecialTag(buf []byte) bool {
	return bytes.HasPrefix(buf, []byte{'<', '!'})
}

// scanFullTag will return end index of the current tag.
//
// It might return error on some broken tags.
func scanFullTag(buf []byte) (int, error) { //nolint:
	return nextTokenStartIndex(buf, '>') + 1, nil
}

func scanSpecial(buf []byte) (int, error) {
	switch {
	case bytes.HasPrefix(buf, cdataPrefix):
		return scanCDATADeclaration(buf)
	case bytes.HasPrefix(buf, docTypePrefix):
		return scanDoctypeDeclaration(buf)
	case bytes.HasPrefix(buf, commentPrefix):
		return scanComment(buf)
	default:
		return 0, fmt.Errorf("unknown declaration: %s", buf[:NextNonSpaceIndex(buf)])
	}
}

func scanCDATADeclaration(buf []byte) (int, error) {
	endIdx := bytes.Index(buf, cdataSuffix)
	if endIdx == -1 {
		return 0, errors.New("no CDATA suffix found")
	}

	return endIdx + cdataSufLen, nil
}

func scanDoctypeDeclaration(buf []byte) (int, error) {
	closeBracket := nextTokenStartIndex(buf, ']')
	if closeBracket == -1 {
		return nextTokenStartIndex(buf, '>'), nil
	}

	return closeBracket + nextTokenStartIndex(buf[closeBracket:], '>'), nil
}

func scanComment(buf []byte) (int, error) {
	idx := bytes.Index(buf, commentSuffix)
	if idx == -1 {
		return 0, errors.New("comment does not have closing suffix")
	}

	return idx + len(commentSuffix), nil
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

	for i := 1; i < len(buf); i++ {
		if !isNameChar(rune(buf[i])) {
			return i
		}
	}

	return 1
}

// nextTokenStartIndex checks that in current buffer there is always visible start of next tag.
//
// Function will check range buf[1:] to skip first byte, which can(and will be) be a tag start token.
//
// Value of searchByte must be '<' or '>' or other escapable XML character.
//
// If len(buf) < 1 then -1 will be returned
// If searchByte was not found in the buf - 0 will be returned, otherwise index of searchByte + 1 will be returned.
func nextTokenStartIndex(buf []byte, searchByte byte) int {
	if len(buf) < 1 {
		return -1
	}

	return bytes.IndexByte(buf[1:], searchByte) + 1
}
