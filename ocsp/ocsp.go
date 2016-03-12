package main

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/ocsp"
)

func getIssuer(cert *x509.Certificate) (*x509.Certificate, error) {
	if len(cert.IssuingCertificateURL) == 0 {
		return nil, fmt.Errorf("No AIA information available, can't get issuer")
	}
	resp, err := http.Get(cert.IssuingCertificateURL[0])
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(body)
	if block == nil {
		return nil, fmt.Errorf("no PEM data found")
	}
	issuer, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return issuer, nil
}

func req(fileName string) error {
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(contents)
	if block == nil {
		return fmt.Errorf("no PEM data found")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	issuer, err := getIssuer(cert)
	if err != nil {
		return fmt.Errorf("getting issuer: %s", err)
	}
	req, err := ocsp.CreateRequest(cert, issuer, nil)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s%s\n", cert.OCSPServer[0], base64.StdEncoding.EncodeToString(req))
	log.Printf("Fetching %s", url)
	httpResp, err := http.Get(url)
	if err != nil {
		return err
	}
	respBytes, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}
	resp, err := ocsp.ParseResponse(respBytes, issuer)
	if err != nil {
		return err
	}
	log.Printf("Good response:\n")
	log.Printf("  Status %d\n", resp.Status)
	log.Printf("  SerialNumber %036x\n", resp.SerialNumber)
	log.Printf("  ProducedAt %s\n", resp.ProducedAt)
	log.Printf("  ThisUpdate %s\n", resp.NextUpdate)
	log.Printf("  NextUpdate %s\n", resp.NextUpdate)
	log.Printf("  RevokedAt %s\n", resp.RevokedAt)
	log.Printf("  RevocationReason %d\n", resp.RevocationReason)
	log.Printf("  SignatureAlgorithm %s\n", resp.SignatureAlgorithm)
	log.Printf("  Extensions %#v\n", resp.Extensions)
	return nil
}

func main() {
	for _, f := range os.Args[1:] {
		err := req(f)
		if err != nil {
			log.Fatal(err)
		}
	}
}
