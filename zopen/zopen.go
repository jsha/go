// zopen offers an io.ReadCloser that decompresses .gz files as it reads them,
// and passes through files that don't end in .gz.
package zopen

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
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

// Open acts exactly like os.Open when filename does not end in ".gz". When
// filename ends in ".gz", Open  returns a File object that gives the
// uncompressed contents of the file.
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

// OpenMany will call Open on each of the filenames, and return a chan string
// that produces lines from those files. Lines from different files may be
// interleaved nondeterministically.
func OpenMany(filenames []string) (<-chan string, error) {
	// Don't open all the files at once or we'll get "too many open files"
	// also it would be wasteful of memory.
	concurrency := runtime.NumCPU()
	sem := make(chan bool, concurrency)

	// Results will be sent through here.
	ch := make(chan string)
	// This is how we know when all the goroutines are done.
	var wg sync.WaitGroup

	go func() {
		for _, fn := range filenames {
			sem <- true
			handle, err := Open(fn)
			if err != nil {
				fmt.Fprintf(os.Stderr, "opening %q: %s\n", fn, err)
				return
			}
			wg.Add(1)
			go process(handle, ch, &wg, sem)
		}

		wg.Wait()
		for i := 0; i < cap(sem); i++ {
			sem <- true
		}
		close(ch)
	}()

	return ch, nil
}

func process(f *File, ch chan<- string, wg *sync.WaitGroup, sem <-chan bool) {
	defer f.Close()
	defer func() { <-sem }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		ch <- scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading input:", err)
	}
	wg.Done()
}
