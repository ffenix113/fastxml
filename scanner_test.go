package fastxml

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestScanTag(t *testing.T) {
	data := `<table ID="lineitem">
<T><L_ORDERKEY>1</L_ORDERKEY><L_PARTKEY>1552</L_PARTKEY>q</T></table>
 `

	sc := bufio.NewScanner(strings.NewReader(data))
	sc.Split((&Parser{}).ScanTag)

	for sc.Scan() {
		fmt.Printf("%q\n", sc.Text())
	}

	// Output:
	// "<a>"
	// "a"
	// "</a>"
	// "\n"
	// "<dd a='b'>"
	// "  aa a <\n"
	// "</dd>"
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

	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()

	var lines int

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lines = 0

		p := NewParser(buf, false)

		for {
			_, err := p.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				b.Fatal(err.Error())
			}
			//b.Log(s.Text())
			lines++
		}
	}

	assert.Equal(b, 3068929, lines)
}

func BenchmarkSTDXML(b *testing.B) {
	buf := prepareFileBuf(b, "testdata/large.xml")
	reader := bytes.NewReader(buf)

	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()

	var lines int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lines = 0
		reader.Seek(0, io.SeekStart)

		dec := xml.NewDecoder(reader)

		for {
			_, err := dec.Token()
			if err != nil {
				if err == io.EOF {
					break
				}
				b.Fatal(err.Error())
			}
			//b.Log(s.Text())
			lines++
		}

		//if err := s.Err(); err != nil {
		//	b.Fatalf("scan err: %s", err.Error())
		//}
	}

	assert.Equal(b, 3068929, lines)
}

func prepareFileBuf(t testing.TB, filePath string) []byte {
	file, err := os.Open(filePath)
	require.NoError(t, err)

	size, err := file.Seek(0, io.SeekEnd)
	require.NoError(t, err)
	file.Seek(0, io.SeekStart)

	buf := make([]byte, size)
	io.ReadFull(file, buf)

	file.Close()

	return buf
}
