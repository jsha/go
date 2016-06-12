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

var server = flag.String("server", "127.0.0.1:53", "DNS server")
var c = new(dns.Client)

func tryAll(name string) error {
	labels := strings.Split(name, ".")

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	_, _, err := c.Exchange(m, *server)
	if err != nil {
		return fmt.Errorf("for A: err")
	}

	for i := 0; i < len(labels); i++ {
		err = try(strings.Join(labels[i:], "."))
		if err != nil {
			return fmt.Errorf("for CAA: err")
		}
	}
	return nil
}

func try(name string) error {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeCAA)
	_, _, err := c.Exchange(m, *server)
	if err != nil {
		return err
	} else {
		return nil
	}
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
