package fastxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"fastxml/testdata"

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

	startTag := tag.(*StartToken)

	var result = map[string]string{
		"id":   "1",
		"attr": "222'2",
	}

	found := map[string]string{}
	for {
		attrName, attrVal, err := startTag.NextAttribute()
		if err != nil {
			require.ErrorIs(t, io.EOF, err)

			break
		}

		found[attrName] = attrVal
	}

	require.Equal(t, result, found)
}

func TestParser_Next(t *testing.T) {
	data := `<ab> some data in between</ab><![CDATA[<tag>  ]]><!---comment- --><a><br/>
<br /> end value 
`

	mustResult := []string{
		`*fastxml.StartToken: &{"ab" ""}`,
		`*fastxml.CharData: &" some data in between"`,
		`*fastxml.EndElement: &{{"" "ab"}}`,
		`*fastxml.CharData: &"<tag>  "`,
		`*fastxml.Comment: &"-comment- "`,
		`*fastxml.StartToken: &{"a" ""}`,
		`*fastxml.StartToken: &{"br" ""}`,
		`*fastxml.EndElement: &{{"" "br"}}`,
		`*fastxml.CharData: &"\n"`,
		`*fastxml.StartToken: &{"br" ""}`,
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

func TestParser_DecodeToken(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		result xml.Token
		err    string
	}{
		{
			name:   "proper comment",
			input:  `<!-- testing chardata with a string of sample legal char except '<' and '&' nor does it contain sequence "]]>" -->`,
			result: Comment(` testing chardata with a string of sample legal char except '<' and '&' nor does it contain sequence "]]>" `),
		},
		{
			name:   "empty valid comment",
			input:  `<!---->`,
			result: Comment(""),
		},
		{
			name:  "small invalid comment",
			input: `<!--->`,
			err:   "decode token: index position 6: comment is not properly formatted",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			token, err := NewParser([]byte(test.input), false).Next()
			if test.err != "" {
				require.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.result, indirectValue(token))
		})
	}
}

func TestParser_Peek(t *testing.T) {
	input := `<a/>`

	p := NewParser([]byte(input), false)

	mustGet := &StartToken{Name: "a"}

	for i := 0; i < 5; i++ {
		peeked, err := p.Peek()
		require.NoError(t, err)
		require.Equal(t, mustGet, peeked)
	}

	next, err := p.Next()
	require.NoError(t, err)
	require.Equal(t, mustGet, next)
}

func TestIBM_XMLSuite(t *testing.T) {
	descFilePath := path.Join(testdata.PackagePath(t), "testdata/suite/ibm/ibm_oasis_valid.xml")

	descData, err := os.ReadFile(descFilePath)
	require.NoError(t, err)

	p := NewParser(descData, false)

	for {
		token, err := p.Next()
		if errors.Is(err, io.EOF) {
			break
		} else {
			require.NoError(t, err)
		}

		start, ok := token.(*StartToken)
		if !ok {
			continue
		}

		_, _, err = start.NextAttribute()
		require.NoError(t, err)

		runXMLGroup(t, p)
	}
}

func runXMLGroup(t *testing.T, p *Parser) {
	for {
		token, err := p.Next()
		require.NoError(t, err)

		if end, ok := token.(*EndElement); ok && end.Name.Local == "TESTCASES" {
			return
		}

		start, ok := token.(*StartToken)
		if !ok {
			continue
		}

		var testFileName string

		for {
			attrName, attrValue, err := start.NextAttribute()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				require.NoError(t, err)
			}

			switch attrName {
			case "URI":
				testFileName = attrValue
			case "ENTITIES":
				if attrValue != "none" {
					return
				}
			}
		}

		if testFileName != "" {
			runXMLTest(t, testFileName)
		}
	}
}

func runXMLTest(t *testing.T, filePath string) {
	filePath = path.Join(testdata.PackagePath(t), "testdata/suite/ibm", filePath)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err, filePath)

	p := NewParser(data, false)

	stdP := xml.NewDecoder(bytes.NewReader(data))

	for {
		tkn, err := p.Next()
		stdToken, stdErr := stdP.Token()

		if errors.Is(err, io.EOF) {
			break
		} else {
			require.NoError(t, err, filePath)
		}

		if stdErr != nil && strings.Contains(stdErr.Error(), "entity &") {
			continue
		}

		require.NoError(t, stdErr, filePath)
		equalTokens(t, tkn, stdToken)

		if start, ok := tkn.(*StartToken); ok {
			if !start.HasAttributes() {
				continue
			}

			for {
				_, _, err := start.NextAttribute()
				if errors.Is(err, io.EOF) {
					return
				} else {
					require.NoError(t, err, filePath)
				}
			}
		}
	}
}

func equalTokens(tb testing.TB, tkn, stdToken xml.Token) {
	require := require.New(tb)

	switch typd := tkn.(type) {
	case *StartToken:
		std, ok := stdToken.(xml.StartElement)
		require.True(ok)

		require.Equal(std.Name.Local, typd.Name)

		if !typd.HasAttributes() {
			require.Empty(std.Attr)
			return
		}

		for _, attr := range std.Attr {
			val, err := typd.GetAttribute(attr.Name.Local)

			require.NoError(err)
			require.Equal(attr.Value, val)
		}
	case *CharData:
		std, ok := stdToken.(xml.CharData)
		require.True(ok)

		require.Equal([]byte(std), []byte(*typd))
	case *Comment:
		std, ok := stdToken.(xml.Comment)
		require.True(ok)

		require.Equal([]byte(std), []byte(*typd))
	case *EndElement:
		std, ok := stdToken.(xml.EndElement)
		require.True(ok)

		require.Equal(std.Name.Local, typd.Name.Local)
	default:
		tb.Logf("unknown token type: %T, std type is %T", typd, stdToken)
	}
}

func indirectValue(val interface{}) interface{} {
	if val == nil {
		return nil
	}

	return reflect.Indirect(reflect.ValueOf(val)).Interface()
}
