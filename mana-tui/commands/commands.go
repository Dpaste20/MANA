package commands

import "strings"

type Result struct {
	Notification string

	NewSessionID string

	ViewportMessage string

	RestoredMessages []string

	// ActiveAgents holds the resolved agent slugs set by /talk.
	// e.g. ["airi"], ["airi","zephyr"], ["airi","zephyr"] for "all"
	ActiveAgents []string

	IsError bool
}

func Dispatch(input string, currentSessionID string, messages []string) (Result, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return Result{}, false
	}

	parts := strings.SplitN(input[1:], " ", 2)
	name := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch name {
	case "save-session", "save":
		return SaveSession(args, currentSessionID, messages)
	case "resume-session", "resume":
		return ResumeSession(args)
	case "list-sessions", "sessions":
		return ListSessions()
	case "talk":
		return TalkCommand(args)

	default:
		return Result{}, false
	}
}
