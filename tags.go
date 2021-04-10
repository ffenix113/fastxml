package fastxml

type StartTag struct {
	Name    string
	attrBuf []byte
	attrIdx int
}

func (t *StartTag) NextAttribute() (string, string, error) {
	if attrBufLen := len(t.attrBuf); attrBufLen == 0 || t.attrIdx > attrBufLen {
		return "", "", nil
	}

	return "", "", nil
}
