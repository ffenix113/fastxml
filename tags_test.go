package fastxml

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartToken_NextAttribute(t *testing.T) {
	// Duplicate attribute name is present to signify that
	// this parser, at least as of now, will not check
	// if duplicate attribute values are present.
	input := `<a a='1' b="2" c="3" c="4" />`

	attr, err := NewParser([]byte(input), false).Next()
	require.NoError(t, err)

	startToken := attr.(*StartToken)

	mustAttrs := [][2]string{
		{"a", "1"},
		{"b", "2"},
		{"c", "3"},
		{"c", "4"},
	}

	for i := 0; i < len(mustAttrs); i++ {
		name, val, err := startToken.NextAttribute()
		require.NoError(t, err)

		require.Equal(t, mustAttrs[i], [2]string{name, val})
	}
	// Verify that no more attributes are present
	_, _, err = startToken.NextAttribute()
	require.Equal(t, io.EOF, err, "unexpected attributes are present")
}
