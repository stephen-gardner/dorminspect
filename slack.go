package main

import (
	"log"

	"github.com/nlopes/slack"
)

type outgoing struct {
	api       *slack.Client
	channelID string
	msg       string
}

func (out *outgoing) send() {
	_, _, err := out.api.PostMessage(out.channelID, slack.MsgOptionText(out.msg, false))
	if err != nil {
		log.Println(err)
	}
}
