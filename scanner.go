package fastxml

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"
)

// ScanTag is a SplitFunc that is intended to be used with bufio.Scanner.
func (p *Parser) ScanTag(data []byte, atEOF bool) (advance int, token []byte, err error) {
	//fmt.Println(len(data), atEOF)
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
	if len(buf) == 0 {
		return 0, nil
	}

	var endIdx int
	for {
		rn, size := utf8.DecodeRune(buf[endIdx:])
		if rn == utf8.RuneError {
			switch size {
			case 0:
				return endIdx, nil
			case 1:
				return endIdx, fmt.Errorf("invalid rune on index %d", endIdx)
			}
		}

		if !isValidChar(rn) || rn == '<' {
			return endIdx, nil
		}

		endIdx += size
	}
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

// scanTillCharDataEnd returns index of last byte of char data.
func scanTillCharDataEnd(buf []byte) int {
	if len(buf) == 0 {
		return 0
	}

	var endIdx int
	for {
		rn, size := utf8.DecodeRune(buf[endIdx:])
		if !isValidChar(rn) {
			return endIdx
		}

		endIdx += size
	}
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

	idx := bytes.IndexByte(buf[1:], searchByte)
	if idx == -1 {
		return -1
	}

	return idx + 1
}
