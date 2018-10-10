package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	irc "github.com/fluffle/goirc/client"
	fglog "github.com/fluffle/goirc/logging/glog"
	"github.com/golang/glog"
)

var postURL = flag.String("post", "", "URL to post to")
var channel = flag.String("channel", "", "Channel to join")
var nick = flag.String("nick", "", "Channel to join")

type payload struct {
	Text      string `json:"text"`
	IconEmoji string `json:"icon_emoji"`
}

func main() {
	flag.Parse()
	if *postURL == "" || *channel == "" || *nick == "" {
		log.Fatalf("All flags are required")
	}
	fglog.Init()
	cfg := irc.NewConfig(*nick)
	cfg.SSL = true
	cfg.Server = "irc.freenode.net:7000"
	cfg.SSLConfig = &tls.Config{
		ServerName: "irc.freenode.net",
	}
	cfg.NewNick = func(n string) string { return n + "^" }
	c := irc.Client(cfg)

	c.HandleFunc(irc.CONNECTED, func(conn *irc.Conn, line *irc.Line) {
		glog.Infof("Connected")
		conn.Join(*channel)
	})
	c.HandleFunc(irc.JOIN, func(conn *irc.Conn, line *irc.Line) {
		if line.Nick == *nick {
			glog.Infof("Joined %s", line.Args[0])
		}
	})
	c.HandleFunc(irc.DISCONNECTED, func(conn *irc.Conn, line *irc.Line) {
		glog.Infof("Disconnected: %s", *line)
		time.Sleep(10 * time.Second)
		for {
			err := c.Connect()
			if err != nil {
				glog.Errorf("Connection error: %s", err)
				continue
			}
			break
		}
	})
	c.HandleFunc(irc.PRIVMSG, func(conn *irc.Conn, line *irc.Line) {
		msg := strings.TrimPrefix(strings.Join(line.Args, " "), *channel)
		result := fmt.Sprintf("<%s>%s\n", line.Nick, msg)
		glog.Infof(result)
		body, err := json.Marshal(payload{
			Text:      result,
			IconEmoji: ":horse:",
		})
		if err != nil {
			glog.Errorf("Err %s", err)
		}
		resp, err := http.Post(*postURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			glog.Errorf("%s", err)
		}
		resp.Body.Close()
	})
	err := c.Connect()
	if err != nil {
		glog.Errorf("Err %s", err)
	}

	select {}
}
