package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentConfig holds the runtime config for a single agent.
type AgentConfig struct {
	Slug        string
	DisplayName string
	WsURL       string
	StartCmd    string
	WorkDir     string
}

type yamlAgent struct {
	DisplayName string `yaml:"display_name"`
	WsURL       string `yaml:"ws_url"`
	StartCmd    string `yaml:"start_cmd"`
	WorkDir     string `yaml:"work_dir"`
}

type yamlRoot struct {
	Agents map[string]yamlAgent `yaml:"agents"`
}

// loadAgents parses config.yaml and returns the agent map keyed by slug.
func loadAgents(path string) (map[string]*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mana-server config not found: %w", err)
	}

	var raw yamlRoot
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
	}
	if raw.Agents == nil {
		return nil, fmt.Errorf("config.yaml must have a top-level 'agents' key")
	}

	agents := make(map[string]*AgentConfig, len(raw.Agents))
	for slug, v := range raw.Agents {
		if v.WsURL == "" {
			return nil, fmt.Errorf("agent %q is missing 'ws_url' in config.yaml", slug)
		}
		name := v.DisplayName
		if name == "" {
			name = capitalize(slug)
		}
		agents[slug] = &AgentConfig{
			Slug:        slug,
			DisplayName: name,
			WsURL:       v.WsURL,
			StartCmd:    v.StartCmd,
			WorkDir:     v.WorkDir,
		}
	}
	return agents, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
