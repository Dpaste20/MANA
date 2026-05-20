package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *Client) Send(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

type procEntry struct {
	cmd  *exec.Cmd
	done chan struct{}
}

type Server struct {
	agents map[string]*AgentConfig
	status map[string]bool
	procs  map[string]*procEntry
	mu     sync.RWMutex
}

func newServer(agents map[string]*AgentConfig) *Server {
	status := make(map[string]bool, len(agents))
	for slug := range agents {
		status[slug] = false
	}
	return &Server{
		agents: agents,
		status: status,
		procs:  make(map[string]*procEntry),
	}
}

func (s *Server) isOnline(slug string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status[slug]
}

func (s *Server) setOnline(slug string, v bool) {
	s.mu.Lock()
	s.status[slug] = v
	s.mu.Unlock()
}

func (s *Server) startHeartbeat() {
	go func() {
		s.pingAll()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.pingAll()
		}
	}()
}

func (s *Server) pingAll() {
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	for slug, agent := range s.agents {
		go func(slug string, agent *AgentConfig) {
			conn, _, err := dialer.Dial(agent.WsURL, nil)
			if err == nil {
				conn.Close()
				s.setOnline(slug, true)
			} else {
				log.Printf("Heartbeat ping failed for %s: %v", slug, err)
				s.setOnline(slug, false)
			}
		}(slug, agent)
	}
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "MANA",
		"status":  "healthy",
		"agents":  s.agentInfoMap(),
	})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.agentInfoMap())
}

func (s *Server) agentInfoMap() map[string]interface{} {
	m := make(map[string]interface{}, len(s.agents))
	for slug, a := range s.agents {
		m[slug] = map[string]string{
			"display_name": a.DisplayName,
			"ws_url":       a.WsURL,
		}
	}
	return m
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	log.Printf("Client connected: %s", r.RemoteAddr)

	client := &Client{conn: conn}

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var data map[string]interface{}
		if err := json.Unmarshal(raw, &data); err != nil {
			client.Send(map[string]interface{}{"type": "error", "message": "invalid JSON"})
			continue
		}

		switch action, _ := data["action"].(string); action {
		case "stop_speech":
			client.Send(map[string]interface{}{"type": "speech_stopped"})

		case "check_online":
			s.handleCheckOnline(client, data)

		case "wake_agent":
			s.handleWake(client, stringsFromData(data["agents"]))

		default:
			route(s.agents, client, data)
		}
	}

	log.Printf("Client disconnected: %s", r.RemoteAddr)
}

func (s *Server) handleCheckOnline(client *Client, data map[string]interface{}) {
	toCheck := stringsFromData(data["agents"])
	if len(toCheck) == 0 || containsStr(toCheck, "all") {
		toCheck = agentSlugs(s.agents)
	}

	var lines []string
	for _, slug := range toCheck {
		slug = strings.ToLower(slug)
		a, ok := s.agents[slug]
		if !ok {
			lines = append(lines, fmt.Sprintf("❌ Unknown agent: **%s**", slug))
			continue
		}
		icon := "Offline 🔴"
		if s.isOnline(slug) {
			icon = "Online 🟢"
		}
		lines = append(lines, fmt.Sprintf("• **%s** (`%s`): %s", a.DisplayName, slug, icon))
	}

	content := "**Agent Status:**\n\n" + strings.Join(lines, "\n")
	client.Send(map[string]interface{}{"type": "start"})
	client.Send(map[string]interface{}{
		"type":       "chunk",
		"content":    content,
		"agent_slug": "mana",
		"agent_name": "Mana",
	})
	client.Send(map[string]interface{}{"type": "end", "token_count": 0, "generation_time": 0.0})
}

func (s *Server) handleWake(client *Client, agentsToWake []string) {
	if len(agentsToWake) == 0 || containsStr(agentsToWake, "all") {
		agentsToWake = agentSlugs(s.agents)
	}

	var valid, unknown []string
	for _, slug := range agentsToWake {
		slug = strings.ToLower(slug)
		if _, ok := s.agents[slug]; ok {
			valid = append(valid, slug)
		} else {
			unknown = append(unknown, slug)
		}
	}

	client.Send(map[string]interface{}{"type": "start"})

	if len(unknown) > 0 {
		client.Send(map[string]interface{}{
			"type":       "chunk",
			"content":    fmt.Sprintf("⚠️  Unknown agent(s): %s\n\n", strings.Join(unknown, ", ")),
			"agent_slug": "mana",
			"agent_name": "Mana",
		})
	}

	if len(valid) == 1 {
		s.wakeSingle(client, valid[0])
	} else {
		client.Send(map[string]interface{}{
			"type":       "chunk",
			"content":    fmt.Sprintf("Waking **%d agents** in parallel...\n\n", len(valid)),
			"agent_slug": "mana",
			"agent_name": "Mana",
		})
		var wg sync.WaitGroup
		for _, slug := range valid {
			wg.Add(1)
			go func(slug string) {
				defer wg.Done()
				s.wakeSingle(client, slug)
			}(slug)
		}
		wg.Wait()
	}

	client.Send(map[string]interface{}{"type": "end", "token_count": 0, "generation_time": 0.0})
}

func (s *Server) wakeSingle(client *Client, slug string) {
	agent := s.agents[slug]
	name := agent.DisplayName

	chunk := func(text string) {
		client.Send(map[string]interface{}{
			"type":       "chunk",
			"content":    text,
			"agent_slug": "mana",
			"agent_name": "Mana",
		})
	}

	if s.isOnline(slug) {
		chunk(fmt.Sprintf("• **%s** is already online 🟢\n", name))
		return
	}

	if agent.StartCmd == "" {
		chunk(fmt.Sprintf("• **%s** has no `start_cmd` configured ❌\n", name))
		return
	}

	s.mu.RLock()
	entry, hasEntry := s.procs[slug]
	s.mu.RUnlock()

	if hasEntry {
		select {
		case <-entry.done:

		default:
			chunk(fmt.Sprintf("• **%s** is already starting up ⏳ — use `/online` to check status\n", name))
			return
		}
	}

	workDir := ""
	if agent.WorkDir != "" {
		home, _ := os.UserHomeDir()
		workDir = strings.ReplaceAll(agent.WorkDir, "~", home)
	}

	chunk(fmt.Sprintf("• Waking **%s**...\n", name))

	cmd := exec.Command("sh", "-c", agent.StartCmd)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		chunk(fmt.Sprintf("• **%s** failed to spawn: `%v` ❌\n", name, err))
		return
	}

	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()

	s.mu.Lock()
	s.procs[slug] = &procEntry{cmd: cmd, done: done}
	s.mu.Unlock()

	const (
		deadline      = 15.0
		pollInterval  = 500 * time.Millisecond
		progressEvery = 3.0
	)
	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	elapsed := 0.0
	nextProgress := progressEvery

	for elapsed < deadline {
		time.Sleep(pollInterval)
		elapsed += 0.5

		if elapsed >= nextProgress {
			chunk(fmt.Sprintf("  Still waiting... (%ds)\n", int(elapsed)))
			nextProgress += progressEvery
		}

		select {
		case <-done:
			chunk(fmt.Sprintf("• **%s** process exited prematurely ❌\n", name))
			return
		default:
		}

		conn, _, err := dialer.Dial(agent.WsURL, nil)
		if err == nil {
			conn.Close()
			s.setOnline(slug, true)
			chunk(fmt.Sprintf("• **%s** is online 🟢  _(boot time: %.1fs)_\n", name, elapsed))
			return
		}
	}

	chunk(fmt.Sprintf("• **%s** did not come up within %.0fs ❌\n", name, deadline))
}

func agentSlugs(agents map[string]*AgentConfig) []string {
	slugs := make([]string, 0, len(agents))
	for slug := range agents {
		slugs = append(slugs, slug)
	}
	return slugs
}

func stringsFromData(v interface{}) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
