package fastxml

import "io"

type StartElement struct {
	Name    string
	attrBuf []byte
}

func (s *StartElement) NextAttribute() (string, string, error) {
	if len(s.attrBuf) <= 4 {
		return "", "", io.EOF
	}

	attrName, attrVal, skipIdx, err := decodeTagAttribute(s.attrBuf)
	if skipIdx != -1 {
		s.attrBuf = s.attrBuf[skipIdx:]
	}

	return attrName, attrVal, err
}
