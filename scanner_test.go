package fastxml

import (
	"bufio"
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
<T><L_ORDERKEY>1</L_ORDERKEY><L_PARTKEY>1552</L_PARTKEY>q`

	sc := bufio.NewScanner(strings.NewReader(data))
	sc.Split(ScanTag)

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
		{"", `<aa dad=aa>\< this is a char data<`, 33},
		{"", `<aa dad=aa>\<\<\<<`, 17},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.result, nextTokenStartIndex([]byte(test.stringData), '<'))
		})
	}
}

func BenchmarkScanTag(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(32295475)

	file, err := os.Open("testdata/large.xml")
	require.NoError(b, err)

	var lines int

	for i := 0; i < b.N; i++ {
		lines = 0
		file.Seek(0, io.SeekStart)

		s := bufio.NewScanner(file)
		s.Split(ScanTag)

		for s.Scan() {
			b.Log(s.Text())
			lines++
		}

		if err := s.Err(); err != nil {
			b.Fatalf("scan err: %s", err.Error())
		}
	}

	assert.Equal(b, 100, lines)
}
