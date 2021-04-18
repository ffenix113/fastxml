package fastxml

import (
	"encoding/xml"
	"io"
)

// This types are defined to "replace" std XML types in favor of custom ones.
// In future this can be beneficial because it will allow to do more things with them.
type (
	CharData   xml.CharData   // "text between tags"
	Comment    xml.Comment    // <!-- comment -->
	Directive  xml.Directive  // <!directive>
	EndElement xml.EndElement // </some_tag>
	ProcInst   xml.ProcInst   // <?xml encoding="UTF-8" ?>
)

// StartElement is current implementation of start tag type.
type StartElement struct {
	Name    string
	attrBuf []byte
}

func (s *StartElement) NextAttribute() (attrName, attrVal string, err error) {
	if len(s.attrBuf) <= 4 {
		return "", "", io.EOF
	}

	var skipIdx int
	attrName, attrVal, skipIdx, err = decodeTagAttribute(s.attrBuf)

	if skipIdx != -1 {
		s.attrBuf = s.attrBuf[skipIdx:]
	}

	return
}
