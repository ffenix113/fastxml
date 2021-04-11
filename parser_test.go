package fastxml

import (
	"errors"
	"fmt"
	"io"
	"testing"

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

func TestParser_Next(t *testing.T) {
	data := `<ab> some data in between</ab><!---comment- --><a><br/>
<br /> end value 
`

	mustResult := []string{
		`*xml.StartElement: &{{"" "ab"} []}`,
		`*xml.CharData: &" some data in between"`,
		`*xml.EndElement: &{{"" "ab"}}`,
		`*xml.Comment: &"-comment- "`,
		`*xml.StartElement: &{{"" "a"} []}`,
		`*xml.StartElement: &{{"" "br"} []}`,
		`*xml.EndElement: &{{"" "br"}}`,
		`*xml.CharData: &"\n"`,
		`*xml.StartElement: &{{"" "br"} []}`,
		`*xml.EndElement: &{{"" "br"}}`,
		`*xml.CharData: &" end value \n"`,
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
