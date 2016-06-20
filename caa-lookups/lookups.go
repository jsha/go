package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/miekg/dns"
	prom "github.com/prometheus/client_golang/prometheus"
)

var debugAddr = flag.String("debugAddr", ":6363", "Timeout")
var timeout = flag.Duration("timeout", 30*time.Second, "Timeout")
var server = flag.String("server", "127.0.0.1:53", "DNS server")
var proto = flag.String("proto", "udp", "DNS proto (tcp or udp)")
var parallel = flag.Int("parallel", 5, "Number of parallel queries")
var spawnRate = flag.Int("spawnRate", 100, "Rate of spawning goroutines")
var spawnInterval = flag.Duration("spawnInterval", 1*time.Minute, "Interval on which to spawn goroutines")
var c *dns.Client

var (
	resultStats = prom.NewCounterVec(prom.CounterOpts{
		Name: "results",
		Help: "lookup results",
	}, []string{"result"})
	attempts = prom.NewCounter(prom.CounterOpts{
		Name: "attempts",
		Help: "number of lookup attempts",
	})
	successes = prom.NewCounter(prom.CounterOpts{
		Name: "successes",
		Help: "number of lookup successes",
	})
	queryTimes = prom.NewSummaryVec(prom.SummaryOpts{
		Name: "queryTime",
		Help: "amount of time queries take",
	}, []string{"type"})
)

func query(name string, typ uint16) error {
	typStr := dns.TypeToString[typ]
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), typ)
	in, rtt, err := c.Exchange(m, *server)
	queryTimes.With(prom.Labels{"type": typStr}).Observe(float64(rtt))
	if err != nil {
		if ne, ok := err.(*net.OpError); ok && ne.Timeout() {
			err = fmt.Errorf("timeout")
		}
		resultStats.With(prom.Labels{"result": err.Error()}).Add(1)
		return fmt.Errorf("for %s: %s", typStr, err)
	} else if in.Rcode != dns.RcodeSuccess {
		rcodeStr := dns.RcodeToString[in.Rcode]
		resultStats.With(prom.Labels{"result": rcodeStr}).Add(1)
		return fmt.Errorf("for %s: %s", typStr, rcodeStr)
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
	resultStats.With(prom.Labels{"result": "ok"}).Add(1)
	return nil
}

func spawn(names chan string, wg *sync.WaitGroup) {
	for i := 0; i < *parallel; {
		for j := 0; j < *spawnRate; i, j = i+1, j+1 {
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
		time.Sleep(*spawnInterval)
	}
}

func main() {
	flag.Parse()
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Fatal(err)
	}
	if *parallel > int(rLimit.Cur) {
		log.Fatalf("ulimit for nofile lower than -parallel: %d vs %d.",
			rLimit.Cur, *parallel)
	}

	prom.MustRegister(resultStats)
	prom.MustRegister(attempts)
	prom.MustRegister(successes)
	prom.MustRegister(queryTimes)

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
	go spawn(names, &wg)
	http.Handle("/metrics", prom.Handler())
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
