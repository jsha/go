// A command line tool to encode and decode base64, including URL and raw
// formats.
package main

import (
	"encoding/base64"
	"flag"
	"io/ioutil"
	"log"
	"os"
)

var url = flag.Bool("u", false, "Use base64url instead of base64")
var raw = flag.Bool("r", false, "Use raw version (with padding stripped)")
var decodeFlag = flag.Bool("d", false, "Decode")

func main() {
	flag.Parse()
	input, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	var encoding *base64.Encoding
	switch {
	case *url && *raw:
		encoding = base64.RawURLEncoding
	case *url && !*raw:
		encoding = base64.URLEncoding
	case !*url && *raw:
		encoding = base64.RawStdEncoding
	case !*url && !*raw:
		encoding = base64.StdEncoding
	}
	var output []byte
	if *decodeFlag {
		output = make([]byte, encoding.EncodedLen(len(input)))
		_, err = encoding.Decode(output, input)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		output = make([]byte, encoding.DecodedLen(len(input)))
		encoding.Encode(output, input)
	}
	os.Stdout.Write(output)
}
