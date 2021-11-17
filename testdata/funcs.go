package testdata

import (
	"path"
	"runtime"
	"testing"
)

func PackagePath(tb testing.TB) string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		tb.Fatal("cannot get file path")
	}

	return path.Dir(file)
}
