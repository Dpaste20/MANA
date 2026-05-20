package main

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
)

func resolveAgents(agents map[string]*AgentConfig, requested []string) ([]*AgentConfig, []string) {
	seen := make(map[string]bool)
	var resolved []*AgentConfig
	var unknown []string

	for _, slug := range requested {
		slug = strings.ToLower(slug)
		if seen[slug] {
			continue
		}
		seen[slug] = true
		if a, ok := agents[slug]; ok {
			resolved = append(resolved, a)
		} else {
			unknown = append(unknown, slug)
		}
	}
	return resolved, unknown
}

func extractMentions(message string, available []string) ([]string, map[string]string) {
	if message == "" {
		return nil, nil
	}

	// Build the pattern: @(slug1|slug2|...|all), longest first to avoid prefix ambiguity
	candidates := make([]string, len(available)+1)
	copy(candidates, available)
	candidates[len(available)] = "all"
	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i]) > len(candidates[j])
	})

	escaped := make([]string, len(candidates))
	for i, s := range candidates {
		escaped[i] = regexp.QuoteMeta(s)
	}
	re := regexp.MustCompile(`(?i)@(` + strings.Join(escaped, "|") + `)\b`)

	locs := re.FindAllStringSubmatchIndex(message, -1)
	if len(locs) == 0 {
		return nil, nil
	}

	var parts []string
	prev := 0
	for _, loc := range locs {
		parts = append(parts, message[prev:loc[0]])
		parts = append(parts, message[loc[2]:loc[3]])
		prev = loc[1]
	}
	parts = append(parts, message[prev:])

	generalContext := strings.TrimSpace(parts[0])

	reLeading := regexp.MustCompile(`(?i)^(and|&|,)\s+`)
	reTrailing := regexp.MustCompile(`(?i)\s+(and|&|,)$`)

	agentTexts := make(map[string][]string)
	var mentionOrder []string

	for i := 1; i+1 < len(parts); i += 2 {
		slug := strings.ToLower(parts[i])
		chunk := strings.TrimSpace(parts[i+1])
		chunk = reLeading.ReplaceAllString(chunk, "")
		chunk = reTrailing.ReplaceAllString(chunk, "")
		chunk = strings.TrimSpace(chunk)

		if _, exists := agentTexts[slug]; !exists {
			agentTexts[slug] = nil
			mentionOrder = append(mentionOrder, slug)
		}
		if chunk != "" {
			agentTexts[slug] = append(agentTexts[slug], chunk)
		}
	}

	// Handle @all: expand to every available agent, share the combined text
	allText := ""
	var ordered []string
	seen := make(map[string]bool)

	if texts, ok := agentTexts["all"]; ok {
		allText = strings.TrimSpace(strings.Join(texts, " "))
		delete(agentTexts, "all")
		for _, a := range available {
			if !seen[a] {
				ordered = append(ordered, a)
				seen[a] = true
			}
		}
	}

	// Add explicitly mentioned agents (preserving mention order)
	for _, slug := range mentionOrder {
		if slug == "all" {
			continue
		}
		if !seen[slug] && containsStr(available, slug) {
			ordered = append(ordered, slug)
			seen[slug] = true
		}
	}

	combined := generalContext
	if allText != "" {
		if combined != "" {
			combined += "\n\n" + allText
		} else {
			combined = allText
		}
	}

	final := make(map[string]string, len(ordered))
	for _, slug := range ordered {
		specific := strings.TrimSpace(strings.Join(agentTexts[slug], " "))
		switch {
		case combined != "" && specific != "":
			final[slug] = combined + "\n\n" + specific
		case specific == "":
			final[slug] = combined
		default:
			final[slug] = specific
		}
	}

	return ordered, final
}

// route dispatches an incoming client message to one or more agent backends.
func route(agents map[string]*AgentConfig, client *Client, data map[string]interface{}) {
	message, _ := data["message"].(string)

	available := make([]string, 0, len(agents))
	for slug := range agents {
		available = append(available, slug)
	}

	sort.Strings(available)
	mentionedSlugs, agentMsgs := extractMentions(message, available)

	var requested []string
	_, hasAgentsField := data["agents"]
	if raw, ok := data["agents"]; ok && raw != nil {
		if arr, ok := raw.([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					requested = append(requested, s)
				}
			}
		}
	}

	if len(mentionedSlugs) > 0 {
		requested = mentionedSlugs
	}

	if len(requested) == 0 {
		client.Send(map[string]interface{}{
			"type":    "error",
			"message": "No agent selected. Use /talk <agent> or @mention an agent.",
		})
		return
	}

	resolved, unknown := resolveAgents(agents, requested)

	if len(resolved) == 0 {
		client.Send(map[string]interface{}{
			"type": "error",
			"message": fmt.Sprintf(
				"Unknown agent(s): %s. Available: %s",
				strings.Join(unknown, ", "),
				strings.Join(available, ", "),
			),
		})
		return
	}

	if len(unknown) > 0 {
		client.Send(map[string]interface{}{
			"type":    "chunk",
			"content": fmt.Sprintf("⚠️  Skipped unknown agent(s): %s\n\n", strings.Join(unknown, ", ")),
		})
	}

	upstreamPayload := make(map[string]interface{}, len(data))
	for k, v := range data {
		if k != "agents" {
			upstreamPayload[k] = v
		}
	}

	forceLabel := !hasAgentsField

	if len(resolved) == 1 {
		routeSingle(client, resolved[0], upstreamPayload, agentMsgs, forceLabel)
	} else {
		routeFanout(client, resolved, upstreamPayload, agentMsgs)
	}
}

func routeSingle(
	client *Client,
	agent *AgentConfig,
	payload map[string]interface{},
	agentMsgs map[string]string,
	forceLabel bool,
) {
	log.Printf("Single route → %s", agent.Slug)

	ch := make(chan QueueItem, 64)

	agentPayload := copyMap(payload)
	if msg, ok := agentMsgs[agent.Slug]; ok && msg != "" {
		agentPayload["message"] = msg
	}

	go (&AgentProxy{Agent: agent, Payload: agentPayload, Out: ch, Label: forceLabel}).Run()

	start := time.Now()
	var totalTokens int
	var generationTime float64

	for {
		item := <-ch

		if item.Msg == nil {
			generationTime = time.Since(start).Seconds()
			break
		}

		switch item.Msg["type"] {
		case "end":
			if tc, ok := item.Msg["token_count"].(float64); ok {
				totalTokens = int(tc)
			}
			if gt, ok := item.Msg["generation_time"].(float64); ok {
				generationTime = gt
			} else {
				generationTime = time.Since(start).Seconds()
			}

			goto done

		default:
			client.Send(item.Msg)
		}
	}

done:
	client.Send(map[string]interface{}{
		"type":            "end",
		"token_count":     totalTokens,
		"generation_time": generationTime,
	})
}

func routeFanout(
	client *Client,
	agents []*AgentConfig,
	payload map[string]interface{},
	agentMsgs map[string]string,
) {
	slugs := make([]string, len(agents))
	for i, a := range agents {
		slugs[i] = a.Slug
	}
	log.Printf("Fan-out route → %v", slugs)

	ch := make(chan QueueItem, 64*len(agents))

	pending := make(map[string]bool, len(agents))
	for _, a := range agents {
		pending[a.Slug] = true
		agentPayload := copyMap(payload)
		if msg, ok := agentMsgs[a.Slug]; ok && msg != "" {
			agentPayload["message"] = msg
		}
		go (&AgentProxy{Agent: a, Payload: agentPayload, Out: ch, Label: true}).Run()
	}

	client.Send(map[string]interface{}{"type": "start"})

	start := time.Now()
	var totalTokens int

	for len(pending) > 0 {
		item := <-ch
		msg := item.Msg

		if msg == nil {
			delete(pending, item.Agent)
			continue
		}

		switch msg["type"] {
		case "start":

		case "end":
			if tc, ok := msg["token_count"].(float64); ok {
				totalTokens += int(tc)
			}
			delete(pending, item.Agent)

		case "error":
			errMsg, _ := msg["message"].(string)
			client.Send(map[string]interface{}{
				"type":    "chunk",
				"content": fmt.Sprintf("\n%s\n", errMsg),
			})

		default:
			client.Send(msg)
		}
	}

	client.Send(map[string]interface{}{
		"type":            "end",
		"token_count":     totalTokens,
		"generation_time": time.Since(start).Seconds(),
	})
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
