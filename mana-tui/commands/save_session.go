package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type SavedSession struct {
	SaveName  string   `json:"save_name"`
	SessionID string   `json:"session_id"`
	SavedAt   string   `json:"saved_at"`
	SavedAtTS int64    `json:"saved_at_ts"`
	Messages  []string `json:"messages"`
}

func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".airi", "sessions")
	return dir, os.MkdirAll(dir, 0755)
}

func slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = regexp.MustCompile(`[^\w\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`[\s_]+`).ReplaceAllString(s, "-")
	if s == "" {
		return fmt.Sprintf("session-%d", time.Now().Unix())
	}
	return s
}

func SaveSession(args, currentSessionID string, messages []string) (Result, bool) {
	var saveName string
	if args == "" {
		saveName = slugify(currentSessionID)
	} else {
		saveName = slugify(args)
	}

	dir, err := sessionsDir()
	if err != nil {
		return Result{
			IsError:         true,
			ViewportMessage: fmt.Sprintf("❌ Could not create sessions directory: %v", err),
			Notification:    "Save failed",
		}, true
	}

	save := SavedSession{
		SaveName:  saveName,
		SessionID: currentSessionID,
		SavedAt:   time.Now().Format("2006-01-02 15:04:05"),
		SavedAtTS: time.Now().Unix(),
		Messages:  messages,
	}

	data, _ := json.MarshalIndent(save, "", "  ")
	path := filepath.Join(dir, saveName+".json")

	if err := os.WriteFile(path, data, 0644); err != nil {
		return Result{
			IsError:         true,
			ViewportMessage: fmt.Sprintf("❌ Failed to write save file: %v", err),
			Notification:    "Save failed",
		}, true
	}

	return Result{
		ViewportMessage: fmt.Sprintf(
			"✅ Session saved as **%s**\n`/resume-session %s` to restore it.",
			saveName, saveName,
		),
		Notification: fmt.Sprintf("Saved → %s", saveName),
	}, true
}
