package fastxml

import (
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
