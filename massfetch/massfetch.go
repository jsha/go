package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var n = flag.Int("n", 1, "Number of requests to make")
var interval = flag.String("interval", "1ns", "Interval between requests")
var method = flag.String("method", "GET", "Request method (GET or POST)")

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Println("provide exactly one URL on command line.")
	}
	intervalDuration, err := time.ParseDuration(*interval)
	if err != nil {
		log.Fatal(err)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}

	ticker := time.NewTicker(intervalDuration)
	var wg sync.WaitGroup
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ticker.C
			var err error
			var resp *http.Response
			switch *method {
			case "GET":
				resp, err = client.Get(args[0])
			case "POST":
				resp, err = client.Post(args[0], "text/plain", strings.NewReader("HI"))
			default:
				fmt.Printf("Method %s not supported]\n", *method)
			}
			if err != nil {
				fmt.Printf("err: %s\n", err)
				return
			}
			defer resp.Body.Close()

			fmt.Printf("HTTP %d\n", resp.StatusCode)
			for k, vv := range resp.Header {
				for _, v := range vv {
					fmt.Printf("%s: %s\n", k, v)
				}
			}
			body, err := ioutil.ReadAll(resp.Body)
			fmt.Printf("\n%s\n", string(body))
		}()
	}
	wg.Wait()
}
