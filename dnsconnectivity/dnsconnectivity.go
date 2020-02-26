package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	prom "github.com/prometheus/client_golang/prometheus"
)

var (
	resultStats = prom.NewCounterVec(prom.CounterOpts{
		Name: "results",
		Help: "query results",
	}, []string{"result", "target", "targetAddr", "qname"})
	queryTimes = prom.NewSummaryVec(prom.SummaryOpts{
		Name:       "queryTime",
		Help:       "amount of time queries take (seconds)",
		Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.02, 0.9: 0.01, 0.99: 0.001},
	}, []string{"target", "targetAddr", "qname"})

	targetsFilename = flag.String("targets", "targets.txt", "File containing space-separated (target, qname) pairs.")
	listenAddr      = flag.String("listenAddr", ":7698", "Address / port to listen on.")
	interval        = flag.String("interval", "5m", "Interval between probes")

	intervalDuration time.Duration
)

func main() {
	flag.Parse()
	var err error
	intervalDuration, err = time.ParseDuration(*interval)
	if err != nil {
		log.Fatal(err)
	}
	prom.MustRegister(resultStats)
	prom.MustRegister(queryTimes)
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(*listenAddr, nil)
	if err := main2(); err != nil {
		log.Fatal(err)
	}
}

type probe struct {
	target string
	qname  string
}

func main2() error {
	probes, err := readTargets()
	if err != nil {
		return err
	}
	log.Print("Probes are:")
	for _, p := range probes {
		log.Printf("target %s, qname %s", p.target, p.qname)
	}

	for _, p := range probes {
		go p.run()
	}
	select {}
}

func readTargets() ([]probe, error) {
	fileHandle, err := os.Open(*targetsFilename)
	if err != nil {
		return nil, err
	}
	defer fileHandle.Close()
	scanner := bufio.NewScanner(fileHandle)

	probes := []probe{}
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) == 0 {
			continue
		}
		tokens := strings.SplitN(text, " ", 2)
		if _, ok := dns.IsDomainName(tokens[0]); !ok {
			return nil, fmt.Errorf("not a domain name or IP: %q", tokens[0])
		}
		if _, ok := dns.IsDomainName(tokens[1]); !ok {
			return nil, fmt.Errorf("not a domain name: %q", tokens[1])
		}
		probes = append(probes, probe{
			target: tokens[0],
			qname:  tokens[1],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return probes, nil
}

func (p probe) run() {
	addrs, err := net.LookupHost(p.target)
	if err != nil {
		log.Fatal(err)
	}
	c := &dns.Client{
		Net:         "udp",
		ReadTimeout: time.Second,
	}
	m := new(dns.Msg)
	m.SetQuestion(p.qname, dns.TypeA)
	for i := 0; ; i++ {
		start := time.Now()
		targetAddr := addrs[i%len(addrs)]
		formattedTarget := fmt.Sprintf("%s (%s)", p.target, targetAddr)
		_, _, err := c.Exchange(m, net.JoinHostPort(targetAddr, "53"))
		duration := time.Since(start)
		if err != nil {
			log.Printf("error asking %s for %q: %s", formattedTarget, p.qname, err)
			resultStats.With(prom.Labels{"result": "err", "target": p.target, "targetAddr": targetAddr, "qname": p.qname}).Add(1)
		} else {
			resultStats.With(prom.Labels{"result": "ok", "target": p.target, "targetAddr": targetAddr, "qname": p.qname}).Add(1)
			log.Printf("probe to %s for %q took %s", formattedTarget, p.qname, duration)
		}
		queryTimes.With(prom.Labels{"target": p.target, "targetAddr": targetAddr, "qname": p.qname}).Observe(duration.Seconds())
		time.Sleep(intervalDuration)
	}
}
