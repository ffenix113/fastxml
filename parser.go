package fastxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

var _ = xml.Header

var (
	ErrNotAValidTag          = errors.New("not a valid tag")
	ErrInvalidClosingElement = errors.New("invalid closing tag")
)

var (
	docTypePrefix = []byte("<!DOCTYPE")
	elementPrefix = []byte("<!ELEMENT")
	attListPrefix = []byte("<!ATTLIST")
)

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
		charData     CharData   // "text between tags"
		comment      Comment    // <!-- comment -->
		directive    Directive  // <!directive>
		startElement StartToken // <some_tag>
		endElement   EndElement // </some_tag>
		procInst     ProcInst   // <?xmxl encoding="UTF-8" ?>
	}
	// currentPointer ALWAYS points to next byte that needs to be processed.
	currentPointer uint32
}

// NewParser will create a parser from input bytes.
//
// Parser MUST own provided buffer, so if input buffer must be modified outside of the parer -
// set `mustCopy` to true and parser will copy full buffer to new slice and will use that.
func NewParser(buf []byte, mustCopy bool) *Parser {
	if mustCopy {
		newBuf := append([]byte(nil), buf...)

		buf = newBuf
	}

	p := Parser{
		buf: buf,
	}

	return &p
}

// Peek can be used to fetch next token without actually advancing parser.
//
// Basically it is wrapper for Parser.Next with state restoration.
func (p *Parser) Peek() (xml.Token, error) {
	lastPos, lastTagName := p.currentPointer, p.lastTagName
	defer func() {
		p.currentPointer, p.lastTagName = lastPos, lastTagName
	}()

	return p.Next()
}

// Next will return next token and error, if any.
//
// Returned token will always be a pointer type.
//
// Caller MUST NOT hold onto returned tokens. Instead, it may store data from them, but don't hold onto pointers.
func (p *Parser) Next() (xml.Token, error) {
	if p.lastTagName != "" {
		token := p.sendSelfClosingEnd()

		p.lastTagName = ""

		return token, nil
	}

	if p.currentPointer >= uint32(len(p.buf)) {
		return nil, io.EOF
	}

	tokenBytes, err := FetchNextToken(p.buf[p.currentPointer:])
	if err != nil {
		return nil, fmt.Errorf("fetch next token: %w", err)
	}

	p.currentPointer += uint32(len(tokenBytes))

	token, err := p.decodeToken(tokenBytes)
	if err != nil {
		return nil, fmt.Errorf("decode token: index position %d: %w", p.currentPointer, err)
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

	if len(buf) < 3 && buf[0] == '<' {
		return nil, ErrNotAValidTag
	}

	switch {
	case buf[0] != '<':
		return p.decodeCharData(buf)
	case buf[0] == '<' && buf[1] == '/':
		return p.decodeClosingTag(buf)
	case buf[0] == '<' && buf[1] == '!' && buf[2] == '-' && buf[3] == '-':
		return p.decodeComment(buf)
	case len(buf) >= 11 && buf[0] == '<' && buf[1] == '!' && buf[2] == '[':
		return p.decodeCdata(buf)
	case buf[0] == '<' && buf[1] == '?':
		return p.decodeProcessInstruction(buf)
	case buf[0] == '<' && buf[1] == '!':
		return p.decodeDeclaration(buf) // Some sort of declaration(ignore, element, attrlist, etc).
	default: // This will be our "catch-all" start tag decoder.
		return p.decodeSimpleTag(buf)
	}
}

func (p *Parser) sendSelfClosingEnd() xml.Token {
	p.innerData.endElement.Name.Local = p.lastTagName

	return &p.innerData.endElement
}

// decodeClosingTag is used to decode closing tag.
func (p *Parser) decodeClosingTag(buf []byte) (xml.Token, error) {
	if len(buf) < 4 || buf[2] == '>' {
		return nil, ErrInvalidClosingElement
	}

	buf = buf[2:]

	nameEndIdx := scanTillWordEnd(buf)
	if nameEndIdx == 0 {
		return nil, ErrInvalidClosingElement
	}

	_ = buf[nameEndIdx] // Remove boundary check
	p.innerData.endElement.Name.Local = unsafeByteToString(buf[:nameEndIdx])

	return &p.innerData.endElement, nil
}

var errCommentNotProperlyFormatted = errors.New("comment is not properly formatted")

func (p *Parser) decodeComment(buf []byte) (xml.Token, error) {
	commentEndIdx := bytes.Index(buf, []byte{'-', '-', '>'})
	if commentEndIdx == -1 || (buf[commentEndIdx-1] == '-' && len(buf) < 7) {
		return nil, errCommentNotProperlyFormatted
	}

	p.innerData.comment = buf[4:commentEndIdx]

	return &p.innerData.comment, nil
}

func (p *Parser) decodeCdata(buf []byte) (xml.Token, error) {
	p.innerData.charData = buf[cdataPrefLen : len(buf)-cdataSufLen]

	return &p.innerData.charData, nil
}

func (p *Parser) decodeCharData(buf []byte) (xml.Token, error) {
	p.innerData.charData = buf

	// Based on https://www.w3.org/TR/xml/#sec-line-ends we
	// always need to normalize carriage returns to new lines.
	// This currently will allocate, but we must conform to
	if bytes.ContainsRune(buf, '\r') {
		p.innerData.charData = p.cleanEOLChars(buf)
	}

	return &p.innerData.charData, nil
}

func (p *Parser) decodeSimpleTag(buf []byte) (xml.Token, error) {
	tagNameIdx := scanTillWordEnd(buf[1:])

	tagName := unsafeByteToString(buf[1 : tagNameIdx+1])

	if buf[len(buf)-2] == '/' {
		p.lastTagName = tagName
	}

	p.innerData.startElement.Name = tagName
	p.innerData.startElement.attrBuf = nil

	buf = buf[tagNameIdx+1:]

	// Skip byte if it is space
	var skipIdx int
	for ; skipIdx < len(buf) && IsHTMLSpaceChar(rune(buf[skipIdx])); skipIdx++ {
	}

	buf = buf[skipIdx:]

	if buf[0] != '>' && buf[0] != '/' {
		p.innerData.startElement.attrBuf = buf
	}

	// Currently we are not supporting attributes.
	// Plan is to have some sort of a function that will parse attributes on demand.

	return &p.innerData.startElement, nil
}

func (p *Parser) decodeDeclaration(buf []byte) (xml.Token, error) {
	switch {
	case bytes.HasPrefix(buf, docTypePrefix),
		bytes.HasPrefix(buf, elementPrefix),
		bytes.HasPrefix(buf, attListPrefix):
	default:
		return nil, fmt.Errorf("unknown declaration: %s", buf[:NextNonSpaceIndex(buf)])
	}

	// FIXME: in doctype it seems that comments should be omitted from resulting value.
	// https://www.w3.org/TR/xml/#dtd
	// Currently, quite a few tests failing because of it.
	p.innerData.directive = buf[2 : len(buf)-1]

	return &p.innerData.directive, nil
}

var emptyBytes = []byte{}

func (p *Parser) decodeProcessInstruction(buf []byte) (xml.Token, error) {
	const lenOfPrefix = 2
	endInstIdx := len(buf) - lenOfPrefix // End of the instruction(end of the tag)
	endTargetIdx := NextSpaceIndex(buf)  // End of the target name

	if endTargetIdx == -1 {
		// No space found in token, nothing to do in this case
		// TODO: this can be better
		p.innerData.procInst.Target = unsafeByteToString(buf[lenOfPrefix:endInstIdx])
		p.innerData.procInst.Inst = emptyBytes

		return &p.innerData.procInst, nil
	}

	beginInstIdx := NextNonSpaceIndex(buf[endTargetIdx:])

	target := buf[lenOfPrefix:endTargetIdx]
	inst := buf[endTargetIdx+beginInstIdx : endInstIdx]

	p.innerData.procInst.Target = unsafeByteToString(target)
	p.innerData.procInst.Inst = inst

	return &p.innerData.procInst, nil
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

	decodedRune, size := utf8.DecodeRune(buf[currPtr:])
	if !isNameStartChar(decodedRune) {
		return currPtr, 0, errors.New("rune is not valid start of name: " + strconv.QuoteRune(decodedRune))
	}

	for {
		currPtr += size

		if currPtr >= len(buf) { // whole buf is proper chars.
			return start, currPtr, nil
		}

		decodedRune, size = utf8.DecodeRune(buf[currPtr:])

		// Check if name is finished
		if IsHTMLSpaceChar(decodedRune) || decodedRune == '=' {
			return start, currPtr, nil
		}

		if !isNameChar(decodedRune) {
			return currPtr, 0, errors.New("rune is not valid name part: " + strconv.QuoteRune(decodedRune))
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

	return start, start + end + 1, nil
}

// NextSpaceIndex will return index on which next rune will be space.
func NextSpaceIndex(buf []byte) (idx int) {
	for idx < len(buf) {
		rn, size := utf8.DecodeRune(buf[idx:])
		if IsHTMLSpaceChar(rn) {
			return
		}

		idx += size
	}

	return -1
}

// NextNonSpaceIndex will return index on which next rune will be non-space.
func NextNonSpaceIndex(buf []byte) (idx int) {
	for idx < len(buf) {
		rn, size := utf8.DecodeRune(buf[idx:])
		if !IsHTMLSpaceChar(rn) {
			return
		}

		idx += size
	}

	return -1
}

func IsHTMLSpaceChar(rn rune) bool {
	switch rn {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

// cleanEOLChars will normalize '\r' and '\r\n' to '\n'.
//
// https://www.w3.org/TR/xml/#sec-line-ends
func (p *Parser) cleanEOLChars(buf []byte) []byte {
	crIdx := bytes.IndexByte(buf, '\r')
	if crIdx == -1 {
		return buf
	}

	var bytesBuf bytes.Buffer

	bytesBuf.Grow(len(buf))

	for {
		crIdx = bytes.IndexByte(buf, '\r')
		if crIdx == -1 {
			bytesBuf.Write(buf)

			return bytesBuf.Bytes()
		}

		bytesBuf.Write(buf[:crIdx])

		if len(buf) > crIdx && buf[crIdx+1] != '\n' {
			bytesBuf.WriteByte('\n')
		}

		buf = buf[crIdx+1:]
	}
}

func isNameStartChar(rn rune) bool {
	return (rn >= 'a' && rn <= 'z') ||
		(rn >= 'A' && rn <= 'Z') ||
		rn == ':' || rn == '_'
}

func isNameChar(rn rune) bool {
	return isNameStartChar(rn) || rn == '-' || rn == '.' ||
		(rn >= '0' && rn <= '9')
}

func unsafeByteToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b)) // nolint:gosec // This is valid and simple conversion.
}
