package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math"
)

func main() {
	body, err := ioutil.ReadFile("malformed.csr")
	if err != nil {
		log.Fatal(err)
	}
	block, _ := pem.Decode(body)
	if block == nil {
		log.Fatal("no block")
	}
	_, err = x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Println("success")
	}
}
