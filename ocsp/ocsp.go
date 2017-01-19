package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jsha/go/ocsp/helper"
)

func main() {
	flag.Parse()
	var errors bool
	for _, f := range flag.Args() {
		_, err := helper.Req(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error for %s: %s\n", f, err)
			errors = true
		}
	}
	if errors {
		os.Exit(1)
	}
}
