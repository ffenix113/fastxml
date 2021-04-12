package fastxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchNextToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		token string
		err   string
	}{
		{name: "", input: "<test> ", token: "<test>"},
		{name: "", input: "</test> ", token: "</test>"},
		{name: "", input: " this is char data <begin>", token: " this is char data "},
		{name: "", input: "<!-- some data --> ", token: "<!-- some data -->"},
		{name: "", input: "<!-- <some data --> ", token: "<!-- <some data -->"},
		{name: "", input: "<![CDATA[data]]> ", token: "<![CDATA[data]]>"},
		{name: "", input: "<![CDATA[]]> ", token: "<![CDATA[]]>"},
		{name: "", input: "<![CDATA[<><><><><>]]> ", token: "<![CDATA[<><><><><>]]>"},
		{name: "", input: "<![CDATA[<greeting>Hello, world!</greeting>]]> ", token: "<![CDATA[<greeting>Hello, world!</greeting>]]>"},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			token, err := FetchNextToken([]byte(test.input))

			if test.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.err)
			}

			assert.Equal(t, test.token, string(token))
		})
	}
}

func TestScanFullCharData(t *testing.T) {
	tests := []struct {
		name       string
		stringData string
		idx        int
		err        string
	}{
		{"", "abcdefg", 7, ""},
		{"", "abc defg", 8, ""},
		{"", "    defg", 8, ""},
		{"", "    defg    ", 12, ""},
		{"", "        ", 8, ""},
		{"", "\n", 1, ""},
		{"", "a<", 1, ""},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			idx, err := scanFullCharData([]byte(test.stringData))

			assert.Equal(t, test.idx, idx)
			if test.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestNextTokenStartIndex(t *testing.T) {
	tests := []struct {
		name       string
		stringData string
		result     int
	}{
		{"", "", -1},
		{"", "<", -1},
		{"", "< ", -1},
		{"", "   ", -1},
		{"", "  dasd   ", -1},
		{"", " <", 1},
		{"", `<aa dad=aa>this is a char data<`, 30},
		{"", `<aa dad=aa><`, 11},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.result, nextTokenStartIndex([]byte(test.stringData), '<'))
		})
	}
}

func BenchmarkScanTag(b *testing.B) {
	buf := prepareFileBuf(b, "testdata/large.xml")

	var lines int

	b.Run("fastxml", func(b *testing.B) {
		b.ResetTimer()
		b.SetBytes(int64(len(buf)))
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			lines = 0

			p := NewParser(buf, false)

			for {
				_, err := p.Next()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					b.Fatal(err.Error())
				}

				lines++
			}
		}

		assert.Equal(b, 3068929, lines)
	})

	b.Run("encoding/xml", func(b *testing.B) {
		b.ResetTimer()
		b.SetBytes(int64(len(buf)))
		b.ReportAllocs()

		reader := bytes.NewReader(buf)

		for i := 0; i < b.N; i++ {
			lines = 0

			reader.Seek(0, io.SeekStart)

			dec := xml.NewDecoder(reader)
			dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
				return input, nil
			}

			for {
				_, err := dec.Token()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					b.Fatal(err.Error())
				}

				lines++
			}
		}

		assert.Equal(b, 3068929, lines)
	})
}

func prepareFileBuf(b *testing.B, filePath string) []byte {
	b.Helper()

	file, err := os.Open(filePath)
	require.NoError(b, err)

	size, err := file.Seek(0, io.SeekEnd)
	require.NoError(b, err)
	file.Seek(0, io.SeekStart)

	buf := make([]byte, size)
	io.ReadFull(file, buf)

	file.Close()

	return buf
}
