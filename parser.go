package fastxml

import (
	"bytes"
	"encoding/xml"
	"errors"
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
	// lastTagName is the last found open tag name.
	// This is necessary for self closing tags. For them there will be two events:
	// startElement and then endElement with the same name.
	lastTagName string
	// innerData holds all available types that will be returned to the caller.
	innerData struct {
		attr         xml.Attr      // <tag name="val" another='val'>
		charData     xml.CharData  // "text between tags"
		comment      xml.Comment   // <!-- comment -->
		directive    xml.Directive // <!directive>
		startElement StartElement  // <some_tag>
		//startElement xml.StartElement // <some_tag>
		endElement xml.EndElement // </some_tag>
		procInst   xml.ProcInst   // <?xmxl encoding="UTF-8" ?>
	}
	// currentPointer ALWAYS points to next byte that needs to be processed.
	currentPointer uint32
	// nextOffset specifies how many bytes were read on last token decoding.
	// This value MUST be added to `currentPointer` before next call to Next.
	nextOffset uint32
}

// NewParser will create a parser from input bytes.
//
// Parser MUST own provided buffer, so if input buffer must be modified outside of the parer -
// set `mustCopy` to true and parser will copy full buffer to new slice and will use that.
func NewParser(buf []byte, mustCopy bool) *Parser {
	if mustCopy {
		newBuf := make([]byte, len(buf))
		copy(newBuf, buf)

		buf = newBuf
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
	if p.lastTagName != "" {
		return p.sendSelfClosingEnd(), nil
	}

	p.currentPointer += p.nextOffset
	if p.currentPointer >= uint32(len(p.buf)) {
		return nil, io.EOF
	}

	tokenBytes, err := FetchNextToken(p.buf[p.currentPointer:])
	if err != nil {
		return nil, err
	}

	p.nextOffset = uint32(len(tokenBytes))

	token, err := p.decodeToken(tokenBytes)
	if err != nil {
		return nil, fmt.Errorf("index position %d: %w", p.currentPointer, err)
	}

	return token, nil
}

// decodeToken receives a buffer for next token and tries to decode it.
//
// Returned token cannot be copied or modified.
// It is valid to copy data from the token.
func (p *Parser) decodeToken(buf []byte) (xml.Token, error) { //nolint:gocyclo,cyclop // Performance matters
	if len(buf) == 0 {
		return nil, io.ErrUnexpectedEOF
	}

	var decoderFunc TokenDecoderFunc

	switch {
	case len(buf) >= 3 && buf[0] == '<' && buf[1] == '/':
		decoderFunc = p.decodeClosingTag
	case len(buf) >= 7 && buf[0] == '<' && buf[1] == '!' && buf[2] == '-' && buf[3] == '-':
		decoderFunc = p.decodeComment
	case len(buf) >= 11 && buf[0] == '<' && buf[1] == '!' && buf[2] == '[':
		decoderFunc = p.decodeCdata
	case len(buf) >= 3 && buf[0] == '<' && buf[1] == '?' && isNameStartChar(rune(buf[3])):
		return nil, errors.New("unknown implementation for processing instruction")
	case buf[0] == '<': // This will be our "catch-all" decoder.
		decoderFunc = p.decodeSimpleTag
	case isValidChar(rune(buf[0])):
		decoderFunc = p.decodeString
	default:
		// We don't know how to handle this case, so return an error.
		return nil, fmt.Errorf("next byte is not valid: %q", buf[0])
	}

	return decoderFunc(buf)
}

func (p *Parser) sendSelfClosingEnd() xml.Token {
	const emptyString = ""

	p.innerData.endElement.Name.Local = p.lastTagName

	p.lastTagName = emptyString

	return &p.innerData.endElement
}

// decodeClosingTag is anything.
func (p *Parser) decodeClosingTag(buf []byte) (xml.Token, error) {
	p.innerData.endElement.Name.Local = unsafeByteToString(buf[2 : len(buf)-1])

	return &p.innerData.endElement, nil
}

func (p *Parser) decodeComment(buf []byte) (xml.Token, error) {
	commentEndIdx := bytes.Index(buf, []byte{'-', '-', '>'})
	if commentEndIdx == -1 || buf[commentEndIdx-1] == '-' {
		return nil, errors.New("comment is not properly formatted")
	}

	p.innerData.comment = buf[4:commentEndIdx]

	return &p.innerData.comment, nil
}

func (p *Parser) decodeCdata(buf []byte) (xml.Token, error) {
	p.innerData.charData = buf[cdataPrefLen : len(buf)-cdataSufLen]

	return &p.innerData.charData, nil
}

func (p *Parser) decodeString(buf []byte) (xml.Token, error) {
	p.innerData.charData = buf

	return &p.innerData.charData, nil
}

func (p *Parser) decodeSimpleTag(buf []byte) (xml.Token, error) {
	tagNameIdx := scanTillWordEnd(buf[1:])

	tagName := unsafeByteToString(buf[1 : tagNameIdx+1])

	if buf[len(buf)-2] == '/' {
		p.lastTagName = tagName
	}

	p.innerData.startElement.Name = tagName
	p.innerData.startElement.attrBuf = buf[tagNameIdx+1:]

	// Currently we are not supporting attributes.
	// Plan is to have some sort of a function that will parse attributes on demand.

	return &p.innerData.startElement, nil
}

func decodeTagAttribute(buf []byte) (string, string, int, error) {
	if len(buf) == 0 || buf[0] == '>' {
		return "", "", -1, nil
	}

	if bytes.IndexByte(buf, '=') == -1 {
		return "", "", 0, errors.New("no equal sign in attributes")
	}

	nonSpaceIdx := NextNonSpaceIndex(buf)
	if buf[nonSpaceIdx] == '>' {
		return "", "", -1, nil
	}

	// Fetch attribute name and position where it ends.
	attrName, endAttrNameIdx, err := NextWord(buf)
	if err != nil {
		return "", "", 0, err
	}

	// Now we need to find equal sign and pass over it.
	equalIdx := nextTokenStartIndex(buf[endAttrNameIdx-1:], '=')

	attrValue, endAttrValueIdx, err := NextQuotedWord(buf[endAttrNameIdx+equalIdx:])
	if err != nil {
		return "", "", 0, err
	}

	// 1 is added to skip index to go over the last quotation mark.
	return attrName, attrValue, endAttrNameIdx + endAttrValueIdx + equalIdx + 1, nil
}

// CopyString will return copy of the input string.
//
// Call this function if you would like to get a copy of a string provided in a Token.
//
// Strings in the returned tokens are only pointers to input buffer.
// As such if data changes in input buffer - values of strings will also change.
func CopyString(s string) string {
	return string([]byte(s))
}

// NextWord will return next word that possibly was located after some spaces.
func NextWord(buf []byte) (word string, endIdx int, err error) {
	var startIdx int

	startIdx, endIdx, err = NextWordIndex(buf)
	if err != nil {
		return "", 0, err
	}

	return unsafeByteToString(buf[startIdx:endIdx]), endIdx, nil
}

// NextQuotedWord will return next quoted word that possibly was located after some spaces.
func NextQuotedWord(buf []byte) (word string, endIdx int, err error) {
	var startIdx int

	startIdx, endIdx, err = NextQuotedWordIndex(buf)
	if err != nil {
		return "", 0, err
	}

	return unsafeByteToString(buf[startIdx+1 : endIdx]), endIdx, nil
}

// NextWordIndex returns two offsets: for start and the end of the word.
// Word is a sequence of alphabetic characters separated by underscore.
//
// On error `start` will hold starting index of the rune that is invalid, `end` will be always 0.
func NextWordIndex(buf []byte) (start, end int, err error) {
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
		if unicode.IsSpace(rn) || rn == '=' {
			return start, currPtr, nil
		}

		if !isNameChar(rn) {
			return currPtr, 0, fmt.Errorf("rune is not valid name part: %c", rn)
		}
	}
}

// NextQuotedWordIndex returns two offsets: for start and the end of the quotes.
// This means that caller MUST do something like `buf[start+1:start+1+end-1]` to get actual word.
//
// Word is a sequence of alphabetic characters separated by underscore.
//
// On error `start` will hold starting index of the rune that is invalid, `end` will be always 0.
//
// Returned indexes will not include quotation mark itself.
//
// Note: current implementation differs from NextWordIndex in a way that
// this function does not validate runes inside of found word.
func NextQuotedWordIndex(buf []byte) (start, end int, err error) {
	start = NextNonSpaceIndex(buf)

	quote := buf[start]
	if quote != '\'' && quote != '"' {
		return 0, 0, errors.New("no quotation mark on the beginning of the word")
	}

	end = bytes.IndexByte(buf[start+1:], quote)
	if end == -1 {
		return 0, 0, errors.New("word is not properly quoted")
	}

	return start, end + 1, nil
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
		(rn >= 'a' && rn <= 'z') ||
		(rn >= 'A' && rn <= 'Z')
}

func isNameChar(rn rune) bool {
	return isNameStartChar(rn) || rn == '-' || rn == '.' ||
		(rn >= '0' && rn <= '9')
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
	return *(*string)(unsafe.Pointer(&b)) // nolint:gosec // This is valid and simple conversion.
}
