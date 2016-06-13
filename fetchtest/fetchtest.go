package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
)

var parallel = flag.Int("parallel", 5, "parallel requests")

func main() {
	flag.Parse()
	names := make(chan string)
	wg := sync.WaitGroup{}
	for i := 0; i < *parallel; i++ {
		go func() {
			for name := range names {
				_, err := http.Get("https://" + name + "/")
				if err != nil {
					fmt.Printf("%s: %s\n", name, err)
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
