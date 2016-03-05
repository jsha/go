// Package reloader provides a method to load a file whenever it changes.
package reloader

import (
	"io/ioutil"
	"os"
	"time"
)

var makeTicker = func() <-chan time.Time {
	return time.Tick(1 * time.Second)
}

// New loads the filename provided, and calls the callback.  It then spawns a
// goroutine to check for updates to that file, calling the callback again with
// any new contents. New returns the error value returned from the first call to
// callback, and discards subsequent return values.  If there is an error
// stat'ing the file or reading it, callback will be called with an error
// parameter.
func New(filename string, callback func([]byte, error) error) error {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	ticker := makeTicker()
	loop := func() {
		for {
			<-ticker

			currentFileInfo, err := os.Stat(filename)
			if err != nil {
				callback(nil, err)
				continue
			}
			if currentFileInfo.ModTime().After(fileInfo.ModTime()) {
				b, err := ioutil.ReadFile(filename)
				if err != nil {
					callback(nil, err)
					continue
				}
				fileInfo = currentFileInfo
				callback(b, nil)
			}
		}
	}
	err = callback(b, nil)
	go loop()
	return err
}
