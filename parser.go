package fastxml

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

var _ = xml.Header

// This is just for reference of available types.
var _ = []interface{}{
	xml.Attr{},         // <tag name="val" another='val'>
	xml.CharData{},     // "text between tags"
	xml.Comment{},      // <!-- comment -->
	xml.Directive{},    // <!directive>
	xml.StartElement{}, // <some_tag>
	xml.EndElement{},   // </some_tag>
	xml.ProcInst{},     // <?xmxl encoding="UTF-8" ?>
	// CDATA			// <![CDATA[...]]> - where '...' is raw string, no parsing.
}

// TokenDecoderFunc if no token can be decoded - error MUST be returned.
type TokenDecoderFunc func([]byte) (xml.Token, int, error)

// Parser currently guarantees to supports only ASCII, UTF8 might chars/sequences be broken.
type Parser struct {
	buf          []byte
	currentDepth int
	// currentPointer ALWAYS points to next byte that needs to be processed.
	currentPointer int
}

func NewParser(buf []byte) *Parser {
	s := bufio.NewScanner(nil)
	s.Split(ScanTag)

	return &Parser{
		buf: buf,
	}
}

func NewParserFromReader(r io.ReadSeeker) *Parser {
	size := seekerSize(r)

	scanner := bufio.NewScanner(r)
	// Allocate large enough slice of bytes for the buffer
	buf := make([]byte, size, size)
	scanner.Buffer(buf, size)
	scanner.Split(ScanTag)

	return nil
}

func seekerSize(s io.Seeker) int {
	size, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		panic(err)
	}
	_, _ = s.Seek(0, io.SeekStart)

	return int(size)
}

func (p *Parser) Next() (xml.Token, error) {
	token, _, err := p.decodeToken(p.buf[p.currentPointer:])

	return token, err
}

func (p *Parser) decodeToken(buf []byte) (xml.Token, int, error) {
	if len(buf) == 0 {
		return nil, 0, io.ErrUnexpectedEOF
	}

	var decodeFunc TokenDecoderFunc
	switch {
	case len(buf) >= 3 && buf[0] == '<' && isNameStartChar(rune(buf[1])):
		decodeFunc = p.decodeSimpleTag
	case len(buf) >= 3 && buf[0] == '<' && buf[1] == '/':
		//decodeFunc = p.decodeClosingTag
	case len(buf) >= 7 && buf[0] == '<' && buf[1] == '!' && buf[2] == '-' && buf[3] == '-':
		//decodeFunc = p.decodeComment
	case len(buf) >= 3 && buf[0] == '<' && buf[1] == '?' && isNameStartChar(rune(buf[3])):
		//decodeFunc = p.decodeProlog // Let's not support this for now.
	case buf[0] == '<': // This will be our "catch-all" decoder.
		decodeFunc = p.decodeTag
	case isNameChar(rune(buf[0])) || unicode.IsSpace(rune(buf[0])):
		decodeFunc = p.decodeString
	default:
		// We don't know how to handle this case, so return an error.
		return nil, p.currentPointer, fmt.Errorf("next byte is not valid: %q", buf[0])
	}

	token, offset, err := decodeFunc(buf)
	if err != nil {
		return nil, 0, err
	}

	p.currentPointer += offset

	return token, p.currentPointer, nil
}

// decodeTag is anything
func (p *Parser) decodeTag(buf []byte) (xml.Token, int, error) {
	return nil, 0, nil
}

func (p *Parser) decodeString(buf []byte) (xml.Token, int, error) {
	return nil, 0, nil
}

func (p *Parser) decodeSimpleTag(buf []byte) (xml.Token, int, error) {
	return nil, 0, nil
}

// NextWordIndex returns two offsets: for start and the end of the word.
// Word is a sequence of alphabetic characters separated by underscore.
//
// This function must be called when `buf` has word in start.
//
// On error `start` will hold starting index of the rune that is invalid, `end` will be always 0.
func NextWordIndex(buf []byte) (start int, end int, err error) {
	start = NextNonSpaceIndex(buf)
	currPtr := start

	rn, size := utf8.DecodeRune(buf[currPtr:])
	if !isNameStartChar(rn) {
		return currPtr, 0, fmt.Errorf("rune is not valid start of name: %c", rn)
	}

	for {
		currPtr += size

		if currPtr >= len(buf) { // whole buf is proper chars.
			return start, currPtr, nil
		}

		rn, size = utf8.DecodeRune(buf[currPtr:])

		// Check if name is finished
		if rn == ' ' {
			return start, currPtr, nil
		}

		if !isNameChar(rn) {
			return currPtr, 0, fmt.Errorf("rune is not valid name part: %c", rn)
		}
	}
}

// NextNonSpaceIndex will return index on which next rune will be non-space.
func NextNonSpaceIndex(buf []byte) (idx int) {
	for {
		rn, size := utf8.DecodeRune(buf[idx:])
		if !unicode.IsSpace(rn) {
			return
		}

		idx += size
	}
}

func isNameStartChar(rn rune) bool {
	return rn == ':' || rn == '_' ||
		('a' <= rn && rn <= 'z') ||
		('A' <= rn && rn <= 'Z')
}

func isNameChar(rn rune) bool {
	return isNameStartChar(rn) || rn == '-' || rn == '.' ||
		('0' <= rn && rn <= '9')
}
