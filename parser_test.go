package fastxml

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestNextWord(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		stringData string
		start, end int
		err        string
	}{
		{
			name:       "simple",
			stringData: "word",
			end:        4,
		},
		{
			name:       "simple with spaces",
			stringData: "  word  ",
			start:      2,
			end:        6,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			if test.data == nil {
				test.data = []byte(test.stringData)
			}

			start, end, err := NextWordIndex(test.data)

			assert.Equal(t, test.start, start)
			assert.Equal(t, test.end, end)

			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNextNonSpaceIndex(t *testing.T) {
	tests := []struct {
		name       string
		stringData string
		idx        int
	}{
		{"simple", "  a", 2},
		{"simple", "a  ", 0},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.idx, NextNonSpaceIndex([]byte(test.stringData)))
		})
	}
}

func TestDecodeTagAttribute(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		attrName, attrVal string
		skipIdx           int
		err               string
	}{
		{"simple", "tag='val'", "tag", "val", 9, ""},
		{"simple another quote", `tag="val"`, "tag", "val", 9, ""},
		{"simple empty value", `tag=""`, "tag", "", 6, ""},
		{"simple no end quote", `tag="`, "", "", 0, "word is not properly quoted"},
		{"simple with space", "tag = 'val'", "tag", "val", 11, ""},
		{"attribute must have name", "='val'", "", "", 0, "rune is not valid start of name: '='"},
		{"attribute must have name", " ='val'", "", "", 0, "rune is not valid start of name: '='"},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			attrName, attrVal, skipIdx, err := decodeTagAttribute([]byte(test.input))

			if test.err != "" {
				require.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.attrName, attrName)
			assert.Equal(t, test.attrVal, attrVal)
			assert.Equal(t, test.skipIdx, skipIdx)
		})
	}
}

func TestStartElement_NextAttribute(t *testing.T) {
	input := []byte(`<tag id='1' attr="222'2">`)
	tag, err := (&Parser{}).decodeSimpleTag(input)

	require.NoError(t, err)

	startTag := tag.(*StartElement)

	for {
		attrName, attrVal, err := startTag.NextAttribute()
		if err != nil {
			require.ErrorIs(t, io.EOF, err)

			return
		}

		t.Logf("%q => %q", attrName, attrVal)
	}
}

func TestParser_Next(t *testing.T) {
	data := `<ab> some data in between</ab><![CDATA[<tag>  ]]><!---comment- --><a><br/>
<br /> end value 
`

	mustResult := []string{
		`*fastxml.StartElement: &{"ab" ""}`,
		`*fastxml.CharData: &" some data in between"`,
		`*fastxml.EndElement: &{{"" "ab"}}`,
		`*fastxml.CharData: &"<tag>  "`,
		`*fastxml.Comment: &"-comment- "`,
		`*fastxml.StartElement: &{"a" ""}`,
		`*fastxml.StartElement: &{"br" ""}`,
		`*fastxml.EndElement: &{{"" "br"}}`,
		`*fastxml.CharData: &"\n"`,
		`*fastxml.StartElement: &{"br" " />"}`,
		`*fastxml.EndElement: &{{"" "br"}}`,
		`*fastxml.CharData: &" end value \n"`,
	}

	p := NewParser([]byte(data), false)

	var results []string

	for {
		token, err := p.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.Log(err.Error())
			}

			break
		}

		results = append(results, fmt.Sprintf("%T: %q", token, token))
	}

	assert.Equal(t, mustResult, results)
}

func TestDecodeClosingTag(t *testing.T) {
	tests := []struct {
		data   string
		result string
		err    string
	}{
		{"</simple>", "simple", ""},
		{"</more_data>", "more_data", ""},
		{"</spaces   	>", "spaces", ""},
		{"</>", "", "invalid closing tag"},
		{"</ 	>", "", "invalid closing tag"},
	}

	p := NewParser(nil, false)

	for _, test := range tests {
		test := test

		t.Run(test.data, func(t *testing.T) {
			token, err := p.decodeClosingTag([]byte(test.data))

			if test.err != "" {
				require.EqualError(t, err, test.err)
				require.Nil(t, token)

				return
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.result, token.(*EndElement).Name.Local)
		})
	}
}
