package fastxml

import (
	"bytes"
	"io"
)

// ScanTag is a SplitFunc that is intended to be used with bufio.Scanner.
func ScanTag(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, io.EOF
	}

	if nextTokenStartIndex(data, '<') == -1 && !atEOF {
		return 0, nil, nil
	}

	//nextWord := NextNonSpaceIndex(data)

	// tagEnd specifies index of end of the tag.
	// Value of 0 tells that not enough data was fed to fetch full tag.
	var tagEnd int

	nextByte := data[0]
	switch {
	case nextByte == '<': // All XML tags start with '<'
		tagEnd, err = scanFullTag(data)
	default: // Treat as text
		tagEnd, err = scanFullCharData(data)
	}

	if err != nil {
		return 0, nil, err
	}

	if tagEnd == 0 {
		return 0, nil, nil
	}

	return tagEnd, data[:tagEnd], nil
}

// scanFullTag
func scanFullTag(buf []byte) (int, error) {
	// not implemented yet
	return nextTokenStartIndex(buf, '>') + 1, nil
}

// scanFulLCharData will return end index of char data.
//
// It is guaranteed that this function will always receive
func scanFullCharData(buf []byte) (int, error) {
	var endIdx int

	for {
		wordEnd := scanTillWordEnd(buf[endIdx:])
		if wordEnd == 0 { // Seems there are no more chars in this word
			spaceEnd := NextNonSpaceIndex(buf[endIdx:])
			if spaceEnd == 0 { // No more name chars and no more spaces - char data is done!
				return endIdx, nil
			}

			endIdx += spaceEnd

			continue
		}
		// Append last index and move on to next chunk of data.
		endIdx += wordEnd
	}
}

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
// searchByte must be '<' or '>' or other escapable XML character.
func nextTokenStartIndex(buf []byte, searchByte byte) int {
	if len(buf) < 1 {
		return -1
	}

	openTagIdx := 1

	for {
		idx := bytes.IndexByte(buf[openTagIdx:], searchByte)
		if idx == -1 { // Not large enough buffer to get to next token beginning.
			return -1
		}

		openTagIdx += idx

		// Simple check that tag start is not escaped
		if buf[openTagIdx-1] != '\\' {
			break
		}

		openTagIdx++ // Advance to next byte
	}

	return openTagIdx
}
