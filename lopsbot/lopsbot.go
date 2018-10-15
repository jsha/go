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

const autoreplyPeriod = time.Hour * 24

var replied map[string]time.Time

var postURL = flag.String("post", "", "URL to post to")
var channel = flag.String("channel", "", "Channel to join")
var nick = flag.String("nick", "", "Nickname for the bot")
var autoreply = flag.String("autoreply", "", "Message to post in reply to any received message (maximum once per 24 hr per nick)")

// postMessage sends the provided message as the text field of an HTTP post
// payload to the configured postURL.
func postMessage(message string) {
	type payload struct {
		Text      string `json:"text"`
		IconEmoji string `json:"icon_emoji"`
	}
	body, err := json.Marshal(payload{
		Text:      message,
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
}

// replyTo will send the autoreply message as a message in the monitored channel
// directed to the specified nick if that nick hasn't received the autoreply
// message in the autoreplyPeriod. If the nick has received the autoreply
// message within the autoreplyPeriod nothing will be done.
func replyTo(conn *irc.Conn, nick string) {
	// If we've already replied and it was within the autoreplyPeriod then don't
	// reply again
	if repliedAt, haveReplied := replied[nick]; haveReplied && time.Since(repliedAt) < autoreplyPeriod {
		return
	}

	replied[nick] = time.Now()
	glog.Infof("%s> %s: %s", *channel, nick, *autoreply)
	conn.Privmsgf(*channel, "%s: %s", nick, *autoreply)
}

func main() {
	flag.Parse()
	if *postURL == "" || *channel == "" || *nick == "" {
		log.Fatalf("The -post, -channel, and -nick flags are required")
	}
	fglog.Init()
	replied = make(map[string]time.Time)
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
		postMessage(result)
		// If there is an autoreply configured, and this message was to a channel
		// and not a direct message to the bot, then reply to it.
		if *autoreply != "" && line.Args[0] == *channel {
			replyTo(conn, line.Nick)
		}
	})
	err := c.Connect()
	if err != nil {
		glog.Errorf("Err %s", err)
	}

	select {}
}
