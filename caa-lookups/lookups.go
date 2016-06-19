package main

import (
	"expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var debugAddr = flag.String("debugAddr", ":6363", "Timeout")
var timeout = flag.Duration("timeout", 30*time.Second, "Timeout")
var server = flag.String("server", "127.0.0.1:53", "DNS server")
var proto = flag.String("proto", "udp", "DNS proto (tcp or udp)")
var parallel = flag.Int("parallel", 5, "Number of parallel queries")
var c *dns.Client

var resultStats = expvar.NewMap("results")
var attempts = expvar.NewInt("attempts")
var successes = expvar.NewInt("successes")

func query(name string, typ uint16) error {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), typ)
	in, _, err := c.Exchange(m, *server)
	if err != nil {
		if ne, ok := err.(*net.OpError); ok && ne.Timeout() {
			err = fmt.Errorf("timeout")
		}
		resultStats.Add(err.Error(), 1)
		return fmt.Errorf("for %s: %s", dns.TypeToString[typ], err)
	} else if in.Rcode != dns.RcodeSuccess {
		rcodeStr := dns.RcodeToString[in.Rcode]
		resultStats.Add(rcodeStr, 1)
		return fmt.Errorf("for %s: %s", dns.TypeToString[typ], rcodeStr)
	}
	return nil
}

func tryAll(name string) error {
	err := query(name, dns.TypeA)
	if err != nil {
		return err
	}

	labels := strings.Split(name, ".")
	for i := 0; i < len(labels); i++ {
		err = query(strings.Join(labels[i:], "."), dns.TypeCAA)
		if err != nil {
			return err
		}
	}
	resultStats.Add("ok", 1)
	return nil
}

func main() {
	flag.Parse()
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	c = &dns.Client{
		Net:         *proto,
		ReadTimeout: *timeout,
	}
	names := make(chan string)
	wg := sync.WaitGroup{}
	for i := 0; i < *parallel; i++ {
		go func() {
			for name := range names {
				attempts.Add(1)
				err := tryAll(name)
				if err != nil {
					fmt.Printf("%s: %s\n", name, err)
				} else {
					successes.Add(1)
				}
				wg.Done()
			}
		}()
	}
	go http.ListenAndServe(*debugAddr, nil)
	for _, name := range strings.Split(string(b), "\n") {
		if name != "" {
			wg.Add(1)
			names <- name
		}
	}
	close(names)
	wg.Wait()
}
