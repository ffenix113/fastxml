package fastxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
	"unsafe"
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
	xml.ProcInst{},     // <?xml encoding="UTF-8" ?>
	// CDATA			// <![CDATA[...]]> - where '...' is raw string, no parsing.
}

// TokenDecoderFunc if no token can be decoded - error MUST be returned.
type TokenDecoderFunc func([]byte) (xml.Token, error)

// Parser currently guarantees to supports only ASCII, UTF8 might chars/sequences be broken.
type Parser struct {
	// buf holds full data to parse.
	buf []byte
	// currentPointer ALWAYS points to next byte that needs to be processed.
	currentPointer int
	// nextOffset specifies how many bytes were read on last token decoding.
	// This value MUST be added to `currentPointer` before next call to Next.
	nextOffset int
	//
	innerData struct {
		attr         xml.Attr         // <tag name="val" another='val'>
		charData     xml.CharData     // "text between tags"
		comment      xml.Comment      // <!-- comment -->
		directive    xml.Directive    // <!directive>
		startElement xml.StartElement // <some_tag>
		endElement   xml.EndElement   // </some_tag>
		procInst     xml.ProcInst     // <?xmxl encoding="UTF-8" ?>
	}
}

// NewParser will create a parser from input bytes.
//
// Parser MUST own provided buffer, so if input buffer must be modified outside of the parer -
// set `mustCopy` to true and parser will copy full buffer to new slice and will use that.
func NewParser(buf []byte, mustCopy bool) *Parser {
	if mustCopy {
		newBuf := make([]byte, len(buf), len(buf))
		copy(newBuf, buf)

		buf = newBuf
	}

	if len(buf) != cap(buf) {
		buf = buf[:len(buf):len(buf)]
	}
	p := Parser{
		buf: buf,
	}

	return &p
}

// Next will return next token and error, if any.
//
// Caller MUST NOT hold onto returned tokens. Instead it may store data from them, but don't hold onto pointers.
func (p *Parser) Next() (xml.Token, error) {
	p.currentPointer += p.nextOffset
	if p.currentPointer >= len(p.buf) {
		return nil, io.EOF
	}

	tokenBytes, err := FetchNextToken(p.buf[p.currentPointer:])
	if err != nil {
		return nil, err
	}

	p.nextOffset = len(tokenBytes)

	//if !p.scanner.Scan() {
	//	return nil, io.EOF // p.scanner.Err()
	//}

	return p.decodeToken(tokenBytes)
}

func (p *Parser) decodeToken(buf []byte) (xml.Token, error) {
	if len(buf) == 0 {
		return nil, io.ErrUnexpectedEOF
	}

	var decodeFunc TokenDecoderFunc
	switch {
	case len(buf) >= 3 && buf[0] == '<' && buf[1] == '/':
		decodeFunc = p.decodeClosingTag
	case len(buf) >= 7 && buf[0] == '<' && buf[1] == '!' && buf[2] == '-' && buf[3] == '-':
		// decodeFunc = p.decodeComment
		panic("unknown implementation for comment")
	case len(buf) >= 11 && buf[0] == '<' && buf[1] == '!' && buf[2] == '[':
		// This is CDATA
		panic("unknown implementation for CDATA")
	case len(buf) >= 3 && buf[0] == '<' && buf[1] == '?' && isNameStartChar(rune(buf[3])):
		// decodeFunc = p.decodeProlog // Let's not support this for now.
		panic("unknown implementation!")
	case buf[0] == '<': // This will be our "catch-all" decoder.
		decodeFunc = p.decodeSimpleTag
	case isValidChar(rune(buf[0])):
		decodeFunc = p.decodeString
	default:
		// We don't know how to handle this case, so return an error.
		return nil, fmt.Errorf("next byte is not valid: %q", buf[0])
	}

	token, err := decodeFunc(buf)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// decodeTag is anything.
func (p *Parser) decodeTag(buf []byte) (xml.Token, error) {
	return nil, nil
}

// decodeClosingTag is anything.
func (p *Parser) decodeClosingTag(buf []byte) (xml.Token, error) {
	p.innerData.endElement.Name.Local = unsafeByteToString(buf[2 : len(buf)-1])

	return &p.innerData.endElement, nil
}

func (p *Parser) decodeString(buf []byte) (xml.Token, error) {
	p.innerData.charData = buf

	return &p.innerData.charData, nil
}

func (p *Parser) decodeSimpleTag(buf []byte) (xml.Token, error) {
	tagNameIdx := scanTillWordEnd(buf[1:])

	if buf[tagNameIdx+1] == '>' {
		p.innerData.startElement.Name.Local = unsafeByteToString(p.buf[p.currentPointer+1 : p.currentPointer+tagNameIdx+1])
		//			buf[1 : tagNameIdx+1])

		return &p.innerData.startElement, nil
	}

	// Currently we are not supporting attributes.
	// Plan is to have some sort of a function that will parse attributes on demand.

	return nil, nil
}

// CopyString will return copy of the input string.
//
// Call this function if you would like to get a copy of a string provided in a Token.
// Otherwise string that would be taken by caller without passing it through this function
// might be changed after last token will be retrieved by Parser.
//
// This is required because in some cases data might be wrapped by `bufio.Scanner`.
func CopyString(s string) string {
	return string([]byte(s))
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

func isValidChar(rn rune) bool {
	return rn == 0x09 ||
		rn == 0x0A ||
		rn == 0x0D ||
		rn >= 0x20 && rn <= 0xD7FF ||
		rn >= 0xE000 && rn <= 0xFFFD ||
		rn >= 0x10000 && rn <= 0x10FFFF
}

func unsafeByteToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
