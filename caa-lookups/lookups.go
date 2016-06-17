package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var server = flag.String("server", "127.0.0.1:53", "DNS server")
var proto = flag.String("proto", "udp", "DNS proto (tcp or udp)")
var c = &dns.Client{
	Net:         *proto,
	ReadTimeout: 30 * time.Second,
}

func query(name string, typ uint16) error {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), typ)
	in, _, err := c.Exchange(m, *server)
	if err != nil {
		return fmt.Errorf("for %s: %s", dns.TypeToString[typ], err)
	} else if in.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("for %s: %s", dns.TypeToString[typ], dns.RcodeToString[in.Rcode])
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
	return nil
}

func main() {
	flag.Parse()
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	names := make(chan string)
	wg := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		go func() {
			for name := range names {
				err := tryAll(name)
				if err != nil {
					fmt.Printf("%s: %s\n", name, err)
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
