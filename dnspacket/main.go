package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/miekg/dns"
)

// Read a binary DNS packet from stdin and print it in standard display form.
func main() {
	err := main2()
	if err != nil {
		log.Fatal(err)
	}
}
func main2() error {
	body, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	msg := new(dns.Msg)
	err = msg.Unpack(body)
	if err != nil {
		return err
	}
	fmt.Println(msg)
	return nil
}
