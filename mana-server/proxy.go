package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type QueueItem struct {
	Agent string
	Msg   map[string]interface{}
}

var proxyDialer = websocket.Dialer{
	HandshakeTimeout: 10 * time.Second,
}

type AgentProxy struct {
	Agent   *AgentConfig
	Payload map[string]interface{}
	Out     chan<- QueueItem
	Label   bool
}

func (p *AgentProxy) Run() {
	defer func() {
		p.Out <- QueueItem{Agent: p.Agent.Slug, Msg: nil}
	}()

	conn, _, err := proxyDialer.Dial(p.Agent.WsURL, nil)
	if err != nil {
		log.Printf("Cannot reach agent %s (%s): %v", p.Agent.Slug, p.Agent.WsURL, err)
		p.pushError("Agent offline")
		return
	}
	defer conn.Close()

	raw, err := json.Marshal(p.Payload)
	if err != nil {
		p.pushError(fmt.Sprintf("payload marshal error: %v", err))
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		p.pushError(fmt.Sprintf("upstream send error: %v", err))
		return
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("Agent %s read error: %v", p.Agent.Slug, err)
				p.pushError(fmt.Sprintf("connection closed: %v", err))
			}
			return
		}

		var m map[string]interface{}
		if err := json.Unmarshal(msg, &m); err != nil {
			continue
		}

		m["agent_slug"] = p.Agent.Slug
		m["agent_name"] = p.Agent.DisplayName

		p.Out <- QueueItem{Agent: p.Agent.Slug, Msg: m}

		if m["type"] == "end" {
			return
		}
	}
}

func (p *AgentProxy) pushError(detail string) {
	p.Out <- QueueItem{
		Agent: p.Agent.Slug,
		Msg: map[string]interface{}{
			"type":    "error",
			"message": fmt.Sprintf("[%s] %s", p.Agent.DisplayName, detail),
		},
	}
}
