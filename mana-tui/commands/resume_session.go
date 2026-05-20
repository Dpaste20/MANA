package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ResumeSession(args string) (Result, bool) {
	if args == "" {
		return ListSessions()
	}

	saveName := slugify(args)

	dir, err := sessionsDir()
	if err != nil {
		return Result{IsError: true, ViewportMessage: fmt.Sprintf("❌ %v", err)}, true
	}

	path := filepath.Join(dir, saveName+".json")
	data, err := os.ReadFile(path)
	if err != nil {

		hint := findSimilar(dir, saveName)
		msg := fmt.Sprintf("❌ No save named **%s** found.", saveName)
		if hint != "" {
			msg += fmt.Sprintf("\nDid you mean `/resume-session %s`?", hint)
		} else {
			msg += "\nRun `/resume-session` to list all saves."
		}
		return Result{IsError: true, ViewportMessage: msg}, true
	}

	var save SavedSession
	if err := json.Unmarshal(data, &save); err != nil {
		return Result{
			IsError:         true,
			ViewportMessage: fmt.Sprintf("❌ Corrupt save file: %v", err),
		}, true
	}

	return Result{
		NewSessionID:     save.SessionID,
		RestoredMessages: save.Messages,
		ViewportMessage: fmt.Sprintf(
			"✅ Resumed **%s** (saved %s)\nContinuing as session `%s`.",
			save.SaveName, save.SavedAt, save.SessionID,
		),
		Notification: fmt.Sprintf("Resumed → %s", save.SaveName),
	}, true
}

func ListSessions() (Result, bool) {
	dir, err := sessionsDir()
	if err != nil {
		return Result{IsError: true, ViewportMessage: fmt.Sprintf("❌ %v", err)}, true
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return Result{
			ViewportMessage: "📂 No saved sessions found.\nUse `/save-session <name>` to save the current conversation.",
		}, true
	}

	type entry struct {
		save SavedSession
	}
	var saves []SavedSession

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s SavedSession
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		saves = append(saves, s)
	}

	sort.Slice(saves, func(i, j int) bool {
		return saves[i].SavedAtTS > saves[j].SavedAtTS
	})

	var lines []string
	lines = append(lines, "📂 **Saved sessions** (newest first):\n")
	for _, s := range saves {
		lines = append(lines, fmt.Sprintf("  • `%s`  —  %s  (id: %s)", s.SaveName, s.SavedAt, s.SessionID))
	}
	lines = append(lines, "\nRun `/resume-session <name>` to restore one.")

	return Result{ViewportMessage: strings.Join(lines, "\n")}, true
}

func findSimilar(dir, query string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if strings.Contains(name, query) || strings.Contains(query, name) {
			return name
		}
	}
	return ""
}
