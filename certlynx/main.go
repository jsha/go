package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/x509"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/go-sql-driver/mysql"
)

var (
	insertCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "inserts",
		Help: "The total number of inserts handled",
	})
)

var dbConnect = flag.String("dbconnect", "", "database to connect to")

func getLogID(db *sql.DB, baseURL string) (int64, error) {
	_, err := db.Exec(`INSERT OR IGNORE INTO logs (url) VALUES(?);`, baseURL)
	if err != nil {
		return -1, err
	}
	rows, err := db.Query(`SELECT id FROM logs WHERE url = ?`, baseURL)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	for rows.Next() {
		var logID int64
		err := rows.Scan(&logID)
		if err != nil {
			return -1, err
		}
		return logID, nil
	}
	return -1, fmt.Errorf("no rows")
}

func getNextIndex(db *sql.DB, logID int64) (int64, error) {
	rows, err := db.Query(`SELECT logIndex FROM logEntries
		WHERE logID = ? ORDER BY logIndex DESC LIMIT 1`, logID)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	for rows.Next() {
		var maxIndex int64
		err := rows.Scan(&maxIndex)
		if err != nil {
			return -1, err
		}
		return maxIndex + 1, nil
	}
	// No rows and no error means start at 0.
	return 0, nil
}

func sync(db *sql.DB, chunks chan<- chunk, baseURL string) error {
	logID, err := getLogID(db, baseURL)
	if err != nil {
		return fmt.Errorf("getting log ID: %s", err)
	}

	nextIndex, err := getNextIndex(db, logID)
	if err != nil {
		return fmt.Errorf("getting max entry for log %d: %s", logID, err)
	}
	log.Printf("next index for log %d (%s) is %d", logID, baseURL, nextIndex)

	client, err := client.New(baseURL, nil, jsonclient.Options{})
	if err != nil {
		return fmt.Errorf("making client: %s", err)
	}

	startIndex := nextIndex
	endIndex := startIndex + 999
	for {
		go func(startIndex, endIndex int64) {
			begin := time.Now()
			entries, err := client.GetEntries(context.TODO(), startIndex, endIndex)
			if err != nil {
				log.Printf("error getting entries %d to %d from log %d (%s): %s", startIndex,
					endIndex, logID, baseURL, err)
			}
			log.Printf("fetched entries %d to %d for log %d (%s) in %s",
				startIndex, endIndex, logID, baseURL, time.Since(begin))

			chunks <- chunk{logID, startIndex, entries}
			log.Printf("stored log %d entries %d through %d", logID, startIndex, endIndex)
		}(startIndex, endIndex)
		startIndex = endIndex + 1
		endIndex = startIndex + 999
	}
	return nil
}

type chunk struct {
	logID      int64
	startIndex int64
	entries    []ct.LogEntry
}

func processChunks(db *sql.DB, chunks <-chan chunk) {
	for ch := range chunks {
		err := processChunk(db, ch)
		if err != nil {
			log.Printf("processing chunk %v: %s", ch, err)
		}
	}
}

func processChunk(db *sql.DB, ch chunk) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	err = saveEntries(tx, ch.logID, ch.startIndex, ch.entries)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func saveEntries(tx *sql.Tx, logID int64, startIndex int64, entries []ct.LogEntry) error {
	for i, v := range entries {
		index := int64(i) + startIndex
		err := saveEntry(tx, logID, index, v)
		if err != nil {
			return fmt.Errorf("saving entry %d/%d: %s", logID, index, err)
		}
	}
	return nil
}

type certificateData struct {
	sha256   []byte
	serial   *big.Int
	notAfter *time.Time
	pre      bool

	issuerID int64
}

func saveEntry(tx *sql.Tx, logID int64, index int64, entry ct.LogEntry) error {
	if index != entry.Index {
		return fmt.Errorf("mismatched indexes: %d calculated vs %d in entry", index, entry.Index)
	}

	var certificateID, issuerID int64
	var pre bool

	var cert *x509.Certificate
	var err error
	if entry.X509Cert != nil {
		cert = entry.X509Cert
	} else if entry.Precert != nil {
		cert = entry.Precert.TBSCertificate
	}

	issuerID, err = getIssuerID(tx, cert.Issuer.String())
	if err != nil {
		return fmt.Errorf("getting issuer ID for %q: %s", cert.Issuer.String(), err)
	}

	sum := sha256.Sum256(cert.Raw)
	dd := certificateData{
		sha256:   sum[:],
		serial:   cert.SerialNumber,
		notAfter: &cert.NotAfter,
		pre:      pre,
	}

	certificateID, err = getCertificateID(tx, issuerID, dd)
	if err != nil {
		return fmt.Errorf("getting certificate ID for (log %d/serial %d): %s", issuerID, dd.serial, err)
	}

	err = storeEntry(tx, logID, index, certificateID)
	if err != nil {
		return fmt.Errorf("saving entry (log %d/index %d): %s", logID, index, err)
	}

	err = storeNames(tx, certificateID, issuerID, cert.NotAfter, cert.DNSNames)

	return nil
}

func storeEntry(tx *sql.Tx, logID, index, certificateID int64) error {
	_, err := tx.Exec(`INSERT INTO logEntries (logID, logIndex, certificateID) VALUES(?, ?, ?)`,
		logID, index, certificateID)
	return err
}

func storeNames(tx *sql.Tx, issuerID, certificateID int64, notAfter time.Time, names []string) error {
	for _, name := range names {
		reversedName := reverseName(name)
		insertCounter.Inc()
		_, err := tx.Exec(`INSERT INTO names (reversedName, notAfter, issuerID, certificateID)
			VALUES(?, ?, ?, ?)`,
			reversedName, notAfter, issuerID, certificateID)
		if err != nil {
			return err
		}
	}
	sum := hashNames(names)
	insertCounter.Inc()
	_, err := tx.Exec(`INSERT INTO fqdnSets (fqdnSetSHA256, notAfter, issuerID, certificateID)
		VALUES(?, ?, ?, ?)`,
		sum, notAfter, issuerID, certificateID)
	if err != nil {
		return err
	}

	return nil
}

func reverseName(domain string) string {
	labels := strings.Split(domain, ".")
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
	}
	return strings.Join(labels, ".")
}

func hashNames(names []string) []byte {
	nameMap := make(map[string]bool, len(names))
	for _, name := range names {
		nameMap[strings.ToLower(name)] = true
	}

	unique := make([]string, 0, len(nameMap))
	for name := range nameMap {
		unique = append(unique, name)
	}
	sort.Strings(unique)
	hash := sha256.Sum256([]byte(strings.Join(unique, ",")))
	return hash[:]
}

func getIssuerID(tx *sql.Tx, issuer string) (int64, error) {
	insertCounter.Inc()
	_, err := tx.Exec(`INSERT OR IGNORE INTO issuers (issuer) VALUES(?)`, issuer)
	if err != nil {
		return -1, err
	}
	rows, err := tx.Query(`SELECT id FROM issuers WHERE issuer = ?`, issuer)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	if !rows.Next() {
		return -1, fmt.Errorf("no rows")
	}
	var issuerID int64
	err = rows.Scan(&issuerID)
	if err != nil {
		return -1, err
	}
	return issuerID, nil
}

func getCertificateID(tx *sql.Tx, issuerID int64, data certificateData) (int64, error) {
	_, err := tx.Exec(`INSERT OR IGNORE INTO certificates
			(sha256, serial, notAfter, pre, issuerID)
			VALUES(?, ?, ?, ?, ?)`,
		data.sha256, data.serial.Bytes(), data.notAfter, data.pre, issuerID)
	if err != nil {
		return -1, err
	}
	rows, err := tx.Query(`SELECT id FROM certificates WHERE sha256 = ?`, data.sha256)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	if !rows.Next() {
		return -1, fmt.Errorf("no rows")
	}
	var certificateID int64
	err = rows.Scan(&certificateID)
	if err != nil {
		return -1, err
	}
	return certificateID, nil
}

func main() {
	flag.Parse()

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)

	db, err := sql.Open("mysql", *dbConnect)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	chunks := make(chan chunk, 20)
	go processChunks(db, chunks)
	for _, v := range flag.Args() {
		url, err := url.Parse(v)
		if err != nil {
			log.Fatalf("not a URL: %q", v)
		}
		if url.Scheme != "https" {
			log.Fatalf("invalid URL scheme %q in %q", url.Scheme, v)
		}
		go func(v string) {
			err := sync(db, chunks, v)
			if err != nil {
				log.Fatal(err)
			}
		}(v)
	}
	select {}
}
