package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/jmhodges/clock"
	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/miekg/dns"
	"github.com/letsencrypt/boulder/bdns"
	"github.com/letsencrypt/boulder/metrics"
)

var parallel = flag.Int("parallel", 5, "parallel requests")
var resolver = flag.String("resolver", "", "resolver")

func getCAASet(ctx context.Context, hostname string, reso bdns.DNSResolver) error {
	hostname = strings.TrimRight(hostname, ".")
	labels := strings.Split(hostname, ".")

	// See RFC 6844 "Certification Authority Processing" for pseudocode.
	// Essentially: check CAA records for the FDQN to be issued, and all
	// parent domains.
	//
	// The lookups are performed in parallel in order to avoid timing out
	// the RPC call.
	//
	// We depend on our resolver to snap CNAME and DNAME records.

	type result struct {
		records []*dns.CAA
		err     error
	}
	results := make([]result, len(labels))

	var wg sync.WaitGroup

	for i := 0; i < len(labels); i++ {
		// Start the concurrent DNS lookup.
		wg.Add(1)
		go func(name string, r *result) {
			r.records, r.err = reso.LookupCAA(ctx, name)
			wg.Done()
		}(strings.Join(labels[i:], "."), &results[i])
	}

	wg.Wait()

	// Return the first result
	for _, res := range results {
		if res.err != nil {
			return res.err
		}
		if len(res.records) > 0 {
			return nil
		}
	}

	// no CAA records found
	return nil
}
func main() {
	flag.Parse()
	reso := bdns.NewDNSResolverImpl(10*time.Second, []string{*resolver}, metrics.NewNoopScope(), clock.Default(), 3)
	names := make(chan string)
	wg := sync.WaitGroup{}
	for i := 0; i < *parallel; i++ {
		go func() {
			for name := range names {
				err := getCAASet(context.Background(), name, reso)
				if err != nil {
					fmt.Printf("%s: %s\n", name, err)
				} else {
					fmt.Printf("%s: OK\n", name)
				}
				wg.Done()
			}
		}()
	}
	reader := bufio.NewScanner(os.Stdin)
	for reader.Scan() {
		name := reader.Text()
		if name != "" {
			wg.Add(1)
			names <- name
		}
	}
	close(names)
	wg.Wait()
}
