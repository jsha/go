package main

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/crypto/ocsp"
)

var method = flag.String("method", "GET", "Method to use for fetching OCSP")
var urlOverride = flag.String("url", "", "URL of OCSP responder to override")

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
	var der []byte
	if block == nil {
		der = body
	} else {
		der = block.Bytes
	}
	issuer, err := x509.ParseCertificate(der)
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
	if len(cert.OCSPServer) == 0 {
		return fmt.Errorf("no ocsp servers in cert")
	}
	encodedReq := base64.StdEncoding.EncodeToString(req)
	var httpResp *http.Response
	ocspServer := cert.OCSPServer[0]
	if *urlOverride != "" {
		ocspServer = *urlOverride
	}
	if *method == "GET" {
		url := fmt.Sprintf("%s%s\n", ocspServer, encodedReq)
		fmt.Printf("Fetching %s\n", url)
		var err error
		httpResp, err = http.Get(url)
		if err != nil {
			return err
		}
	} else if *method == "POST" {
		fmt.Printf("Posting to %s: base64dec(%s)\n", ocspServer, encodedReq)
		var err error
		httpResp, err = http.Post(ocspServer, "application/ocsp-request", bytes.NewBuffer(req))
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("invalid method %s, expected GET or POST", *method)
	}
	for k, v := range httpResp.Header {
		for _, vv := range v {
			fmt.Printf("%s: %s\n", k, vv)
		}
	}
	respBytes, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}
	fmt.Printf("\nDecoding body: %s\n", base64.StdEncoding.EncodeToString(respBytes))
	resp, err := ocsp.ParseResponse(respBytes, issuer)
	if err != nil {
		return err
	}
	fmt.Printf("\n")
	fmt.Printf("Good response:\n")
	fmt.Printf("  Status %d\n", resp.Status)
	fmt.Printf("  SerialNumber %036x\n", resp.SerialNumber)
	fmt.Printf("  ProducedAt %s\n", resp.ProducedAt)
	fmt.Printf("  ThisUpdate %s\n", resp.NextUpdate)
	fmt.Printf("  NextUpdate %s\n", resp.NextUpdate)
	fmt.Printf("  RevokedAt %s\n", resp.RevokedAt)
	fmt.Printf("  RevocationReason %d\n", resp.RevocationReason)
	fmt.Printf("  SignatureAlgorithm %s\n", resp.SignatureAlgorithm)
	fmt.Printf("  Extensions %#v\n", resp.Extensions)
	return nil
}

func main() {
	flag.Parse()
	for _, f := range flag.Args() {
		err := req(f)
		if err != nil {
			fmt.Printf("error: %s\n", err)
			return
		}
	}
}
