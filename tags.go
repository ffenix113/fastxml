package fastxml

import (
	"encoding/xml"
	"io"
	"strings"
)

// MinAttrLen specifies minimum possible of chars for valid attribute.
//
// 1 char name, 1 char equal sign, 2 quotes.
const MinAttrLen = 4

// This types are defined to "replace" std XML types in favor of custom ones.
// In future this can be beneficial because it will allow to do more things with them.
type (
	CharData   xml.CharData   // "text between tags"
	Comment    xml.Comment    // <!-- comment -->
	Directive  xml.Directive  // <!directive>
	EndElement xml.EndElement // </some_tag>
	ProcInst   xml.ProcInst   // <?xml encoding="UTF-8" ?>
)

// StartToken is current implementation of start tag type.
type StartToken struct {
	Name    string
	attrBuf []byte
}

// HasAttributes only specifies if current tag has attributes.
//
// Resulting value cannot be used to check if more attributes are available,
// instead this method answer question "does this tag have attributes".
func (s *StartToken) HasAttributes() bool {
	return s.attrBuf != nil
}

// NextAttribute will return next set of attribute name and value.
// This method will return io.EOF when no more attributes will be returned.
//
// By specification tags should not contain any attributes with
// repeated names (https://www.w3.org/TR/2006/REC-xml11-20060816/#uniqattspec).
// Currently, this parser does not adhere to this requirement,
// meaning that if this parser will parse tag with attributes with same names -
// they still will be returned and no error will be produced.
//
// So tag with these attributes will be properly parsed: <a a='1' a='2'>, with two attributes being returned: a=1, a=2.
func (s *StartToken) NextAttribute() (attrName, attrVal string, err error) {
	if len(s.attrBuf) <= MinAttrLen {
		return "", "", io.EOF
	}

	var skipIdx int
	attrName, attrVal, skipIdx, err = decodeTagAttribute(s.attrBuf)

	if skipIdx != -1 {
		s.attrBuf = s.attrBuf[skipIdx:]
	}

	return
}

type NoSuchAttributeError string

func (a NoSuchAttributeError) Error() string {
	return "no such attribute found: " + string(a)
}

// GetAttribute will return first value attached to an attribute name and true,
// or empty string and false.
func (s *StartToken) GetAttribute(name string) (value string, err error) {
	var (
		nextAttrIdx, skipIdx int
		attrName             string
	)

	for {
		if len(s.attrBuf)-skipIdx <= MinAttrLen {
			return "", NoSuchAttributeError(name)
		}

		attrName, value, skipIdx, err = decodeTagAttribute(s.attrBuf[nextAttrIdx:])
		if err != nil {
			return "", err
		}

		if colIdx := strings.IndexByte(attrName, ':'); colIdx != -1 {
			attrName = attrName[colIdx+1:]
		}

		if attrName == name {
			return value, nil
		}

		if skipIdx != -1 {
			nextAttrIdx += skipIdx
		}
	}
}
