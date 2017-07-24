package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/publicsuffix"

	"github.com/miekg/dns"

	_ "github.com/go-sql-driver/mysql"
)

var server = flag.String("server", "127.0.0.1:53", "DNS server")
var db = flag.String("db", "root:@tcp(172.19.0.3:3306)/boulder_sa_integration", "DB server")
var parallel = flag.Int("parallel", 5, "Number of parallel queries to run")

type client struct {
	*dns.Client
}

type result struct {
	fqdn, ns, nsetld1 string
}

func main() {
	flag.Parse()
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("mysql", *db)
	if err != nil {
		panic(err.Error()) // Just for example purpose. You should use proper error handling instead of panic
	}
	defer db.Close()

	stmt, err := db.Prepare("INSERT nslist SET fqdn=?,ns=?,nsetld1=?")
	if err != nil {
		log.Fatal(err)
	}

	c := client{new(dns.Client)}
	names := make(chan string)
	wg := sync.WaitGroup{}
	for i := 0; i < *parallel; i++ {
		go func() {
			for name := range names {
				nses := c.query(name)
				for _, ns := range nses {
					stmt.Exec(ns.fqdn, ns.ns, ns.nsetld1)
				}
				wg.Done()
			}
		}()
	}
	for _, name := range strings.Split(string(b), "\n") {
		if name != "" {
			wg.Add(1)
			names <- name
		}
	}
	close(names)
	wg.Wait()
}

func (c client) query(name string) []result {
	labels := strings.Split(name, ".")
	for i := 0; i < len(labels)-1; i++ {
		results := c.queryFQDN(strings.Join(labels[i:], "."))
		if len(results) > 0 {
			return results
		}
	}
	return nil
}

func (c client) queryFQDN(name string) (results []result) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeNS)
	in, _, err := c.Exchange(m, *server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", name, err)
	} else {
		for _, ans := range in.Answer {
			if ns, ok := ans.(*dns.NS); ok {
				ns := ns.Ns
				ns = ns[:len(ns)-1]
				etld1, err := publicsuffix.EffectiveTLDPlusOne(ns)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", name, err)
					continue
				}
				results = append(results, result{name, ns, etld1})
			}
		}
		for _, ans := range in.Ns {
			if soa, ok := ans.(*dns.SOA); ok {
				ns := soa.Ns
				ns = ns[:len(ns)-1]
				etld1, err := publicsuffix.EffectiveTLDPlusOne(ns)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", name, err)
					continue
				}
				results = append(results, result{name, ns, etld1})
			}
		}
	}
	return
}
