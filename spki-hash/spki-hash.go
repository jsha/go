package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
)

func print(f string) error {
	certPEM, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("no PEM data found")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(der)
	fmt.Printf("%s: %s\n", f, base64.StdEncoding.EncodeToString(hash[:]))
	return nil
}

func main() {
	for _, f := range os.Args[1:] {
		err := print(f)
		if err != nil {
			fmt.Printf("%s: %s\n", f, err)
		}
	}
}
