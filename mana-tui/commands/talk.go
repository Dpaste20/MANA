package commands

import (
	"fmt"
	"strings"
)

var KnownAgents = map[string]string{
	"airi":   "Airi",
	"zephyr": "Zephyr",
	"itta":   "ITAA",
	"kuber":  "KUBER",
}

func TalkCommand(args string) (Result, bool) {
	args = strings.TrimSpace(args)

	if args == "" {
		return talkUsage(), true
	}

	if strings.ToLower(args) == "all" {
		slugs := allAgentSlugs()
		names := agentDisplayNames(slugs)
		return Result{
			ActiveAgents:    slugs,
			ViewportMessage: fmt.Sprintf("📡 Now talking to **all agents**: %s", strings.Join(names, ", ")),
			Notification:    "→ All agents",
		}, true
	}

	parts := strings.Fields(strings.ToLower(args))
	var resolved []string
	var unknown []string

	for _, p := range parts {
		if _, ok := KnownAgents[p]; ok {

			if !contains(resolved, p) {
				resolved = append(resolved, p)
			}
		} else {
			unknown = append(unknown, p)
		}
	}

	if len(unknown) > 0 && len(resolved) == 0 {
		agentList := knownAgentList()
		return Result{
			IsError: true,
			ViewportMessage: fmt.Sprintf(
				"❌ Unknown agent(s): **%s**\nAvailable agents: %s",
				strings.Join(unknown, ", "), agentList,
			),
		}, true
	}

	names := agentDisplayNames(resolved)
	msg := fmt.Sprintf("📡 Now talking to: **%s**", strings.Join(names, " + "))

	if len(unknown) > 0 {
		msg += fmt.Sprintf("\n⚠️  Skipped unknown agent(s): %s", strings.Join(unknown, ", "))
	}

	notif := "→ " + strings.Join(names, " + ")

	return Result{
		ActiveAgents:    resolved,
		ViewportMessage: msg,
		Notification:    notif,
	}, true
}

func talkUsage() Result {
	var agentLines []string
	for slug, name := range KnownAgents {
		agentLines = append(agentLines, fmt.Sprintf("  • `%s` — %s", slug, name))
	}

	msg := fmt.Sprintf(`**Usage:**
  /talk <agent>              Talk to one agent
  /talk <agent1> <agent2>    Talk to multiple agents simultaneously
  /talk all                  Broadcast to all agents

**Available agents:**
%s`, strings.Join(agentLines, "\n"))

	return Result{ViewportMessage: msg}
}

func allAgentSlugs() []string {
	slugs := make([]string, 0, len(KnownAgents))
	for slug := range KnownAgents {
		slugs = append(slugs, slug)
	}
	return slugs
}

func agentDisplayNames(slugs []string) []string {
	names := make([]string, 0, len(slugs))
	for _, s := range slugs {
		if name, ok := KnownAgents[s]; ok {
			names = append(names, name)
		}
	}
	return names
}

func knownAgentList() string {
	var parts []string
	for slug := range KnownAgents {
		parts = append(parts, "`"+slug+"`")
	}
	return strings.Join(parts, ", ")
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
