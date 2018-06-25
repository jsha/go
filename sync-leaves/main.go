package main

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/tls"
)

type entriesResponse struct {
	Entries []struct {
		LeafInput string `json:"leaf_input"`
	}
}

func fetchInto(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

func sync(baseURL string, start, end int) {
	var entries entriesResponse
	fetchInto(fmt.Sprintf("%s/ct/v1/get-entries?start=%d&end=%d", baseURL, start, end),
		&entries)
	for _, entry := range entries.Entries {
		merkleTreeLeafBytes, err := base64.StdEncoding.DecodeString(entry.LeafInput)
		if err != nil {
			log.Print(err)
		}

		var mtl ct.MerkleTreeLeaf
		tls.Unmarshal(merkleTreeLeafBytes, &mtl)
		if mtl.LeafType != ct.TimestampedEntryLeafType {
			log.Printf("unrecognized leaf type %d", mtl.LeafType)
			continue
		}
		timestampedEntry := mtl.TimestampedEntry
		var der []byte
		switch timestampedEntry.EntryType {
		case ct.X509LogEntryType:
			der = timestampedEntry.X509Entry.Data
		case ct.PrecertLogEntryType:
			der = timestampedEntry.PrecertEntry.TBSCertificate
		default:
			log.Printf("unrecognized entry type %d", timestampedEntry.EntryType)
			continue
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			log.Printf("parsing der: %s", err)
		}
		fmt.Println(timestampedEntry.EntryType, cert.AuthorityKeyId, cert.DNSNames)
	}
}

func main() {
	sync(os.Args[1], 0, 10)
}
