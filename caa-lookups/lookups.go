package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

var parallel = flag.Int("parallel", 5, "parallel requests")
var server = flag.String("server", "127.0.0.1:53", "DNS server")

func main() {
	flag.Parse()
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	c := new(dns.Client)
	names := make(chan string)
	wg := sync.WaitGroup{}
	for i := 0; i < *parallel; i++ {
		go func() {
			for name := range names {
				m := new(dns.Msg)
				m.SetQuestion(dns.Fqdn(name), dns.TypeCAA)
				m.SetEdns0(4096, true)
				in, _, err := c.Exchange(m, *server)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", name, err)
				} else {
					fmt.Printf("%s: %s\n", name, dns.RcodeToString[in.Rcode])
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
