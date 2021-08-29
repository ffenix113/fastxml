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
		{"", "<", 0},
		{"", "< ", 0},
		{"", "   ", 0},
		{"", "  dasd   ", 0},
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
	benchmarks := []struct {
		name string
		file string
	}{
		{"small", "small.xml"},
		{"large", "psd7003.xml"},
	}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			b.Run("fastxml", func(b *testing.B) {
				b.ReportAllocs()

				buf := prepareFileBuf(b, "testdata/"+bench.file)
				b.ResetTimer()
				b.SetBytes(int64(len(buf)))

				for i := 0; i < b.N; i++ {
					p := NewParser(buf, false)

					for {
						tkn, err := p.Next()
						if err != nil {
							if errors.Is(err, io.EOF) {
								break
							}

							b.Fatal(err.Error())
						}

						if startToken, ok := tkn.(*StartToken); ok {
							if startToken.HasAttributes() {
								var err error
								for err == nil {
									_, _, err = startToken.NextAttribute()
								}
							}
						}
					}
				}
			})

			b.Run("encoding/xml", func(b *testing.B) {
				b.SkipNow()

				b.ReportAllocs()

				buf := prepareFileBuf(b, "testdata/"+bench.file)

				b.ResetTimer()
				b.SetBytes(int64(len(buf)))

				reader := bytes.NewReader(buf)

				for i := 0; i < b.N; i++ {
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
					}
				}
			})
		})
	}
}

func TestStartToken_HasAttributes(t *testing.T) {
	tests := []struct {
		name   string
		tag    string
		result bool
	}{
		{"simple no", "<a>", false},
		{"simple no self closing", "<a/>", false},
		{"simple no self closing #2", "<a />", false},
		{"simple no self closing more spaces", "<a       	\n/>", false},
		{"simple yes", "<a tag='2'>", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tag, err := (&Parser{}).decodeSimpleTag([]byte(test.tag))
			require.NoError(t, err)

			startTag := tag.(*StartToken)

			require.Equal(t, test.result, startTag.HasAttributes())
		})
	}
}

func BenchmarkNextTokenStartIndex(b *testing.B) {
	data := []byte("<daadsafyuv att='val' ddd=''>")

	var idx int

	for i := 0; i < b.N; i++ {
		idx = nextTokenStartIndex(data, '=')
	}

	if idx != 15 {
		require.Equal(b, 15, idx)
	}
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
