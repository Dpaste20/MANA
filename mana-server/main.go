package main

import (
	"log"
	"net/http"
)

func main() {
	agents, err := loadAgents("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srv := newServer(agents)
	srv.startHeartbeat()

	log.Println("━━━ MANA Server starting ━━━")
	log.Println("Registered agents:")
	for slug, agent := range agents {
		log.Printf("  %-12s  →  %s", slug, agent.WsURL)
	}
	log.Println("Listening on ws://0.0.0.0:8080/ws/chat")

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleRoot)
	mux.HandleFunc("/agents", srv.handleAgents)
	mux.HandleFunc("/ws/chat", srv.handleWebSocket)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
