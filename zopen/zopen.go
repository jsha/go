// zopen offers an io.ReadCloser that decompresses .gz files as it reads them,
// and passes through files that don't end in .gz.
package zopen

import (
	"compress/gzip"
	"io"
	"os"
	"strings"
)

type File struct {
	io.ReadCloser
	gzipReader *gzip.Reader
}

func (f *File) Close() error {
	if f.gzipReader != nil {
		err := f.gzipReader.Close()
		if err != nil {
			return err
		}
	}
	return f.ReadCloser.Close()
}

func Open(filename string) (*File, error) {
	var f io.ReadCloser
	var err error
	f, err = os.Open(filename)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(filename, ".gz") {
		gzipReader, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		return &File{gzipReader, gzipReader}, nil
	}

	return &File{f, nil}, nil
}
