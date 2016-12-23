package main

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ocsp"
)

var method = flag.String("method", "GET", "Method to use for fetching OCSP")
var urlOverride = flag.String("url", "", "URL of OCSP responder to override")
var tooSoon = flag.Int("too-soon", 76, "If NextUpdate is fewer than this many hours in future, warn.")
var ignoreExpiredCerts = flag.Bool("ignore-expired-certs", false, "If a cert is expired, don't bother requesting OCSP.")

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
	if strings.Join(resp.Header["Content-Type"], "") == "application/x-pkcs7-mime" {
		return parseCMS(body)
	}
	return parse(body)
}

func parse(body []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(body)
	var der []byte
	if block == nil {
		der = body
	} else {
		der = block.Bytes
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

// parseCMS parses certificates from CMS messages of type SignedData.
func parseCMS(body []byte) (*x509.Certificate, error) {
	type signedData struct {
		Version          int
		Digests          asn1.RawValue
		EncapContentInfo asn1.RawValue
		Certificates     asn1.RawValue
	}
	type cms struct {
		ContentType asn1.ObjectIdentifier
		SignedData  signedData `asn1:"explicit,tag:0"`
	}
	var msg cms
	_, err := asn1.Unmarshal(body, &msg)
	cert, err := x509.ParseCertificate(msg.SignedData.Certificates.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing CMS: %s", err)
	}
	return cert, nil
}

func req(fileName string, tooSoonDuration time.Duration) error {
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	cert, err := parse(contents)
	if err != nil {
		return fmt.Errorf("parsing certificate: %s", err)
	}
	if time.Now().After(cert.NotAfter) {
		if *ignoreExpiredCerts {
			return nil
		} else {
			return fmt.Errorf("certificate expired %s ago: %s",
				time.Now().Sub(cert.NotAfter), cert.NotAfter)
		}
	}
	issuer, err := getIssuer(cert)
	if err != nil {
		return fmt.Errorf("getting issuer: %s", err)
	}
	req, err := ocsp.CreateRequest(cert, issuer, nil)
	if err != nil {
		return fmt.Errorf("creating OCSP request: %s", err)
	}
	if len(cert.OCSPServer) == 0 {
		return fmt.Errorf("no ocsp servers in cert")
	}
	encodedReq := base64.StdEncoding.EncodeToString(req)
	var httpResp *http.Response
	ocspServer := cert.OCSPServer[0]
	ocspURL, err := url.Parse(ocspServer)
	if *urlOverride != "" {
		ocspServer = *urlOverride
	}
	if err != nil {
		return fmt.Errorf("parsing URL: %s", err)
	}
	if *method == "GET" {
		ocspURL.Path = encodedReq
		fmt.Printf("Fetching %s\n", ocspURL.String())
		var err error
		httpResp, err = http.Get(ocspURL.String())
		if err != nil {
			return fmt.Errorf("fetching: %s", err)
		}
	} else if *method == "POST" {
		fmt.Printf("Posting to %s: base64dec(%s)\n", ocspServer, encodedReq)
		var err error
		httpResp, err = http.Post(ocspServer, "application/ocsp-request", bytes.NewBuffer(req))
		if err != nil {
			return fmt.Errorf("fetching: %s", err)
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
	if len(respBytes) == 0 {
		return fmt.Errorf("empty reponse body")
	}
	fmt.Printf("\nDecoding body: %s\n", base64.StdEncoding.EncodeToString(respBytes))
	resp, err := ocsp.ParseResponse(respBytes, issuer)
	if err != nil {
		return fmt.Errorf("parsing response: %s", err)
	}
	fmt.Printf("\n")
	fmt.Printf("Good response:\n")
	fmt.Printf("  Status %d\n", resp.Status)
	fmt.Printf("  SerialNumber %036x\n", resp.SerialNumber)
	fmt.Printf("  ProducedAt %s\n", resp.ProducedAt)
	fmt.Printf("  ThisUpdate %s\n", resp.ThisUpdate)
	fmt.Printf("  NextUpdate %s\n", resp.NextUpdate)
	fmt.Printf("  RevokedAt %s\n", resp.RevokedAt)
	fmt.Printf("  RevocationReason %d\n", resp.RevocationReason)
	fmt.Printf("  SignatureAlgorithm %s\n", resp.SignatureAlgorithm)
	fmt.Printf("  Extensions %#v\n", resp.Extensions)
	timeTilExpiry := resp.NextUpdate.Sub(time.Now())
	if timeTilExpiry < tooSoonDuration {
		return fmt.Errorf("NextUpdate is too soon: %s", timeTilExpiry)
	}
	return nil
}

func main() {
	flag.Parse()
	var errors bool
	for _, f := range flag.Args() {
		err := req(f, time.Duration(*tooSoon)*time.Hour)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error for %s: %s\n", f, err)
			errors = true
		}
	}
	if errors {
		os.Exit(1)
	}
}
