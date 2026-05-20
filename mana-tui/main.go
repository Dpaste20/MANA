package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"math/rand"

	"github.com/Dpaste20/mana-tui/commands"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

const manaLogo = `
███╗   ███╗    █████╗    ███╗   ██╗    █████╗
████╗ ████║   ██╔══██╗   ████╗  ██║   ██╔══██╗
██╔████╔██║   ███████║   ██╔██╗ ██║   ███████║
██║╚██╔╝██║   ██╔══██║   ██║╚██╗██║   ██╔══██║
██║ ╚═╝ ██║██╗██║  ██║██╗██║ ╚████║██╗██║  ██║
╚═╝     ╚═╝╚═╝╚═╝  ╚═╝╚═╝╚═╝  ╚═══╝╚═╝╚═╝  ╚═╝

`

var (
	senderStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	aiStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	infoStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	commandStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	recordingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Blink(true)

	userBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("5")).
			Padding(0, 1).
			Margin(0, 0)

	aiBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("2")).
			Padding(0, 1).
			Margin(0, 0)

	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Align(lipgloss.Center)

	helpTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Underline(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	statusConnectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42")).
				Bold(true)

	statusDisconnectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	statusLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	statusValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Bold(true)

	notificationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("63")).
				Bold(true).
				Padding(0, 2)

	acItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("235")).
			Padding(0, 2)

	acSelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("63")).
			Bold(true).
			Padding(0, 2)

	acBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63"))

	fileBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("214")).
			Bold(true).
			Padding(0, 1)

	fileBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Padding(0, 1)

	fpOverlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Background(lipgloss.Color("232")).
			Padding(1, 2)

	fpTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Underline(true)

	fpDirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("75")).
			Bold(true)

	fpFileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	fpDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

	fpSelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("214")).
			Bold(true)

	fpHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)

var knownCommands = []string{
	"/help",
	"/attach",
	"/detach",
	"/save-session",
	"/resume-session",
	"/list-sessions",
	"/talk",
	"/online",
	"/wake",
}

func getMatches(input string) []string {
	if input == "" {
		return nil
	}

	lastIdx := strings.LastIndex(input, " ")
	lastWord := input
	if lastIdx != -1 {
		lastWord = input[lastIdx+1:]
	}

	if lastWord == "" {
		return nil
	}

	lower := strings.ToLower(lastWord)
	var matches []string

	if strings.HasPrefix(lower, "/") && lastIdx == -1 {

		for _, cmd := range knownCommands {
			if strings.HasPrefix(cmd, lower) && cmd != lower {
				matches = append(matches, cmd)
			}
		}
	} else if strings.HasPrefix(lower, "@") {

		candidates := []string{"@all"}
		for slug := range commands.KnownAgents {
			candidates = append(candidates, "@"+slug)
		}

		for _, cand := range candidates {
			if strings.HasPrefix(cand, lower) && cand != lower {
				matches = append(matches, cand)
			}
		}
		sort.Strings(matches)
	}
	return matches
}

func extractMentionedAgents(input string) []string {
	lower := strings.ToLower(input)
	var found []string
	for slug := range commands.KnownAgents {
		re := regexp.MustCompile(`@` + regexp.QuoteMeta(slug) + `\b`)
		if re.MatchString(lower) {
			found = append(found, slug)
		}
	}
	return found
}

func applySuggestion(input, suggestion string) string {
	lastIdx := strings.LastIndex(input, " ")
	if lastIdx == -1 {
		return suggestion + " "
	}
	return input[:lastIdx+1] + suggestion + " "
}

type FileAttachment struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	MimeType string `json:"mime_type"`
}

type AttachedFile struct {
	Name     string
	Path     string
	Content  string
	MimeType string
	Size     int64
}

type ChatRequest struct {
	Message         string           `json:"message,omitempty"`
	SessionID       string           `json:"session_id"`
	SearchKnowledge bool             `json:"search_knowledge"`
	AudioData       string           `json:"audio_data,omitempty"`
	Action          string           `json:"action,omitempty"`
	Files           []FileAttachment `json:"files,omitempty"`
	Agents          []string         `json:"agents,omitempty"`
}

type WSMessage struct {
	Type           string  `json:"type"`
	Content        string  `json:"content"`
	Message        string  `json:"message"`
	Error          string  `json:"error"`
	TokenCount     int     `json:"token_count"`
	GenerationTime float64 `json:"generation_time"`
	AgentSlug      string  `json:"agent_slug"`
	AgentName      string  `json:"agent_name"`
}

type clearNotificationMsg struct{}

type fpItem struct {
	Name     string
	FullPath string
	IsDir    bool
	IsUp     bool
	Mime     string
	Size     int64
}

type model struct {
	conn               *websocket.Conn
	viewport           viewport.Model
	textInput          textinput.Model
	renderer           *glamour.TermRenderer
	messages           []string
	activeStreams      map[string]string
	streamOrder        []string
	err                error
	spinner            spinner.Model
	isLoading          bool
	sessionID          string
	connected          bool
	messageCount       int
	startTime          time.Time
	width              int
	height             int
	generatedTokens    int
	generationStart    time.Time
	generationTime     time.Duration
	tokensPerSecond    float64
	isRecording        bool
	recordCmd          *exec.Cmd
	awaitingVoiceChunk bool
	pendingVoiceInput  string
	isSpeaking         bool
	lastAiResponse     string
	notification       string
	showHelp           bool
	suggestions        []string
	suggestionIdx      int
	attachedFiles      []AttachedFile
	activeAgents       []string
	showFilePicker     bool
	fpDir              string
	fpItems            []fpItem
	fpIdx              int
	inputHistory       []string
	historyIdx         int
}

func mimeFromExt(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".md", ".log", ".csv":
		return "text/plain", true
	case ".pdf":
		return "application/pdf", true
	default:
		return "", false
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func loadAttachedFile(rawPath string) (AttachedFile, error) {
	path := expandHome(strings.TrimSpace(rawPath))

	mime, ok := mimeFromExt(path)
	if !ok {
		return AttachedFile{}, fmt.Errorf("unsupported file type — only .txt and .pdf are accepted")
	}

	info, err := os.Stat(path)
	if err != nil {
		return AttachedFile{}, fmt.Errorf("file not found: %s", path)
	}
	const maxBytes = 20 * 1024 * 1024
	if info.Size() > maxBytes {
		return AttachedFile{}, fmt.Errorf("file too large (max 20 MB): %s", filepath.Base(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return AttachedFile{}, fmt.Errorf("could not read file: %v", err)
	}

	return AttachedFile{
		Name:     filepath.Base(path),
		Path:     path,
		Content:  base64.StdEncoding.EncodeToString(data),
		MimeType: mime,
		Size:     info.Size(),
	}, nil
}

func loadFpDir(dir string) ([]fpItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var dirs, files []fpItem

	for _, e := range entries {

		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fullPath := filepath.Join(dir, e.Name())
		if e.IsDir() {
			dirs = append(dirs, fpItem{
				Name:     e.Name() + "/",
				FullPath: fullPath,
				IsDir:    true,
			})
		} else {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			mime, _ := mimeFromExt(e.Name())
			files = append(files, fpItem{
				Name:     e.Name(),
				FullPath: fullPath,
				Mime:     mime,
				Size:     size,
			})
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	var items []fpItem

	if filepath.Dir(dir) != dir {
		items = append(items, fpItem{
			Name:     "../",
			FullPath: filepath.Dir(dir),
			IsDir:    true,
			IsUp:     true,
		})
	}
	items = append(items, dirs...)
	items = append(items, files...)
	return items, nil
}

func (m model) renderFilePicker() string {
	const maxVisible = 16

	title := fpTitleStyle.Render("📂 File Picker")
	path := fpHintStyle.Render(" " + m.fpDir)

	start := 0
	if m.fpIdx >= maxVisible {
		start = m.fpIdx - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.fpItems) {
		end = len(m.fpItems)
	}

	var rows []string
	for i := start; i < end; i++ {
		item := m.fpItems[i]

		var label string
		switch {
		case item.IsUp:
			label = fpDirStyle.Render("  ↑  ../")
		case item.IsDir:
			label = fpDirStyle.Render("  📁 " + item.Name)
		case item.Mime != "":
			icon := "📄 "
			if item.Mime == "application/pdf" {
				icon = "📕 "
			}
			sizeStr := fmt.Sprintf("%.1f KB", float64(item.Size)/1024)
			if item.Size >= 1024*1024 {
				sizeStr = fmt.Sprintf("%.1f MB", float64(item.Size)/1024/1024)
			}
			label = fpFileStyle.Render(fmt.Sprintf("  %s%-36s %s", icon, item.Name, sizeStr))
		default:
			label = fpDimStyle.Render("  ·  " + item.Name)
		}

		if i == m.fpIdx {

			plain := item.Name
			if item.IsUp {
				plain = "../"
			}
			_ = plain
			label = fpSelectedStyle.Render(label)
		}

		rows = append(rows, label)
	}

	scrollInfo := ""
	if len(m.fpItems) > maxVisible {
		scrollInfo = fpHintStyle.Render(fmt.Sprintf(" %d/%d", m.fpIdx+1, len(m.fpItems)))
	}

	empty := ""
	if len(m.fpItems) == 0 {
		empty = fpDimStyle.Render("  (empty directory)")
	}

	hints := fpHintStyle.Render("↑↓ navigate  Enter select/open  Backspace parent  Esc close")

	innerWidth := m.width - 8
	if innerWidth < 40 {
		innerWidth = 40
	}

	body := fmt.Sprintf("%s%s\n%s\n\n%s%s\n\n%s",
		title, scrollInfo,
		path,
		strings.Join(rows, "\n"),
		empty,
		hints,
	)

	box := fpOverlayStyle.Width(innerWidth).Render(body)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func newSessionID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return "terminal-" + string(b)
}

func initialModel(conn *websocket.Conn) model {
	ti := textinput.New()
	ti.Placeholder = "Use /talk <agent> to select, then start chatting.."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 20

	defaultWidth := 80
	vp := viewport.New(defaultWidth, 20)

	coloredLogo := aiStyle.Render(manaLogo)
	logoDisplay := lipgloss.PlaceHorizontal(defaultWidth, lipgloss.Center, coloredLogo)

	welcomeMsg := infoStyle.Render("\nConnected to Mana Terminal.\n")

	vp.SetContent(logoDisplay + "\n" + welcomeMsg)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(74),
	)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))

	return model{
		conn:               conn,
		textInput:          ti,
		viewport:           vp,
		renderer:           renderer,
		messages:           []string{logoDisplay, welcomeMsg},
		activeStreams:      make(map[string]string),
		streamOrder:        nil,
		spinner:            s,
		isLoading:          false,
		sessionID:          newSessionID(),
		connected:          true,
		messageCount:       0,
		startTime:          time.Now(),
		width:              defaultWidth,
		height:             24,
		generatedTokens:    0,
		generationTime:     0,
		tokensPerSecond:    0.0,
		isRecording:        false,
		awaitingVoiceChunk: false,
		pendingVoiceInput:  "",
		isSpeaking:         false,
		lastAiResponse:     "",
		notification:       "",
		showHelp:           false,
		suggestions:        nil,
		suggestionIdx:      -1,
		attachedFiles:      nil,
		showFilePicker:     false,
		fpDir:              "",
		fpItems:            nil,
		fpIdx:              0,
		inputHistory:       nil,
		historyIdx:         -1,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		waitForIncomingMessage(m.conn),
	)
}

func preprocessMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "### ") {
			lines[i] = "**" + strings.TrimPrefix(line, "### ") + "**"
		} else if strings.HasPrefix(line, "## ") {
			lines[i] = "**" + strings.TrimPrefix(line, "## ") + "**"
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) renderMarkdown(text string) string {
	text = preprocessMarkdown(text)
	tr, err := m.renderer.Render(text)
	if err != nil {
		return text
	}
	return tr
}

func (m model) renderStatusBar() string {
	if m.notification != "" {
		msg := notificationStyle.Render(m.notification)
		return statusBarStyle.Width(m.width).Render(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, msg))
	}

	var connStatus string
	if m.connected {
		connStatus = statusConnectedStyle.Render("● Connected")
	} else {
		connStatus = statusDisconnectedStyle.Render("● Disconnected")
	}

	var recStatus string
	if m.isRecording {
		recStatus = recordingStyle.Render("● Listening")
	} else if m.isSpeaking {
		recStatus = statusLabelStyle.Render("Speaking")
	} else {
		recStatus = statusLabelStyle.Render("Space to Talk")
	}

	msgCount := fmt.Sprintf("%s %s",
		statusLabelStyle.Render("Messages:"),
		statusValueStyle.Render(fmt.Sprintf("%d", m.messageCount)))

	sessionInfo := fmt.Sprintf("%s %s",
		statusLabelStyle.Render("Session:"),
		statusValueStyle.Render(m.sessionID))

	genTokens := fmt.Sprintf("%s %s",
		statusLabelStyle.Render("Tokens:"),
		statusValueStyle.Render(fmt.Sprintf("%d", m.generatedTokens)))

	genTime := fmt.Sprintf("%s %s",
		statusLabelStyle.Render("Time:"),
		statusValueStyle.Render(fmt.Sprintf("%.2fs", m.generationTime.Seconds())))

	genSpeed := fmt.Sprintf("%s %s",
		statusLabelStyle.Render("Speed:"),
		statusValueStyle.Render(fmt.Sprintf("%.2f t/s", m.tokensPerSecond)))

	var agentStatus string
	if len(m.activeAgents) == 0 {
		agentStatus = fmt.Sprintf("%s %s",
			statusLabelStyle.Render("Agent:"),
			statusLabelStyle.Render("none"))
	} else if len(m.activeAgents) == len(commands.KnownAgents) {
		agentStatus = fmt.Sprintf("%s %s",
			statusLabelStyle.Render("Agent:"),
			statusConnectedStyle.Render("All"))
	} else {
		var names []string
		for _, slug := range m.activeAgents {
			if name, ok := commands.KnownAgents[slug]; ok {
				names = append(names, name)
			} else {
				names = append(names, slug)
			}
		}
		agentStatus = fmt.Sprintf("%s %s",
			statusLabelStyle.Render("Agent:"),
			statusConnectedStyle.Render(strings.Join(names, "+")))
	}

	statusContent := lipgloss.JoinHorizontal(lipgloss.Left,
		connStatus,
		"  ",
		agentStatus,
		"  ",
		recStatus,
		"  ",
		msgCount,
		"  ",
		sessionInfo,
		"  ",
		genTokens,
		"  ",
		genTime,
		"  ",
		genSpeed,
	)

	return statusBarStyle.Width(m.width).Render(statusContent)
}

func (m model) getAiHeader() string {
	if len(m.activeAgents) == 1 {
		slug := m.activeAgents[0]
		name := slug
		if n, ok := commands.KnownAgents[slug]; ok {
			name = n
		} else {
			if len(name) > 0 {
				name = strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
			}
		}
		return aiStyle.Render(name + ":")
	} else if len(m.activeAgents) > 1 {
		return aiStyle.Render("Agents:")
	}
	return aiStyle.Render("Mana:")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {

	case tea.KeyMsg:
		if m.showFilePicker {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit

			case tea.KeyEsc, tea.KeyCtrlF:
				m.showFilePicker = false
				return m, nil

			case tea.KeyUp:
				if m.fpIdx > 0 {
					m.fpIdx--
				}
				return m, nil

			case tea.KeyDown:
				if m.fpIdx < len(m.fpItems)-1 {
					m.fpIdx++
				}
				return m, nil

			case tea.KeyBackspace:
				parent := filepath.Dir(m.fpDir)
				if parent != m.fpDir {
					items, err := loadFpDir(parent)
					if err == nil {
						m.fpDir = parent
						m.fpItems = items
						m.fpIdx = 0
					}
				}
				return m, nil

			case tea.KeyEnter:
				if len(m.fpItems) == 0 {
					return m, nil
				}
				item := m.fpItems[m.fpIdx]

				if item.IsDir {
					items, err := loadFpDir(item.FullPath)
					if err != nil {
						m.notification = "✗ " + err.Error()
						return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
							return clearNotificationMsg{}
						})
					}
					m.fpDir = item.FullPath
					m.fpItems = items
					m.fpIdx = 0
					return m, nil
				}

				if item.Mime == "" {
					m.notification = "⚠ Only .txt and .pdf files can be attached"
					return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
						return clearNotificationMsg{}
					})
				}

				af, err := loadAttachedFile(item.FullPath)
				if err != nil {
					m.notification = "✗ " + err.Error()
					return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
						return clearNotificationMsg{}
					})
				}

				for _, existing := range m.attachedFiles {
					if existing.Path == af.Path {
						m.notification = "⚠ Already attached: " + af.Name
						m.showFilePicker = false
						return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
							return clearNotificationMsg{}
						})
					}
				}

				m.attachedFiles = append(m.attachedFiles, af)
				m.showFilePicker = false

				sizeStr := fmt.Sprintf("%.1f KB", float64(af.Size)/1024)
				if af.Size >= 1024*1024 {
					sizeStr = fmt.Sprintf("%.1f MB", float64(af.Size)/1024/1024)
				}
				m.notification = fmt.Sprintf("📎 %s (%s) attached", af.Name, sizeStr)
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearNotificationMsg{}
				})
			}
			return m, nil
		}

		if m.showHelp {
			if msg.Type == tea.KeyCtrlX {
				m.showHelp = false
			}

			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			if m.isRecording && m.recordCmd != nil {
				_ = m.recordCmd.Process.Kill()
			}
			return m, tea.Quit

		case tea.KeyCtrlY:
			if m.lastAiResponse != "" {
				err := clipboard.WriteAll(m.lastAiResponse)
				if err == nil {
					m.notification = "✓ Copied to clipboard"
				} else {
					m.notification = "✗ Copy failed"
				}

				return m, tea.Tick(time.Millisecond*500, func(_ time.Time) tea.Msg {
					return clearNotificationMsg{}
				})
			}
			return m, nil

		case tea.KeyCtrlD:
			if len(m.attachedFiles) > 0 {
				m.attachedFiles = nil
				m.notification = "✓ Attachments cleared"
				return m, tea.Tick(time.Millisecond*800, func(_ time.Time) tea.Msg {
					return clearNotificationMsg{}
				})
			}
			return m, nil

		case tea.KeyCtrlF:
			if m.showFilePicker {
				m.showFilePicker = false
				return m, nil
			}

			startDir, err := os.UserHomeDir()
			if err != nil {
				startDir, _ = os.Getwd()
			}
			items, err := loadFpDir(startDir)
			if err != nil {
				m.notification = "✗ Cannot open " + startDir
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearNotificationMsg{}
				})
			}
			m.fpDir = startDir
			m.fpItems = items
			m.fpIdx = 0
			m.showFilePicker = true
			return m, nil

		case tea.KeyTab:
			if len(m.suggestions) > 0 {
				idx := m.suggestionIdx
				if idx < 0 {
					idx = 0
				}

				m.textInput.SetValue(applySuggestion(m.textInput.Value(), m.suggestions[idx]))
				m.textInput.CursorEnd()
				m.suggestions = nil
				m.suggestionIdx = -1
			}
			return m, nil

		case tea.KeyUp:
			if len(m.suggestions) > 0 {
				m.suggestionIdx--
				if m.suggestionIdx < 0 {
					m.suggestionIdx = len(m.suggestions) - 1
				}
				return m, nil
			}

			if len(m.inputHistory) > 0 {
				if m.historyIdx == -1 {
					m.historyIdx = len(m.inputHistory) - 1
				} else if m.historyIdx > 0 {
					m.historyIdx--
				}
				m.textInput.SetValue(m.inputHistory[m.historyIdx])
				m.textInput.CursorEnd()
				return m, nil
			}

		case tea.KeyDown:
			if len(m.suggestions) > 0 {
				m.suggestionIdx++
				if m.suggestionIdx >= len(m.suggestions) {
					m.suggestionIdx = 0
				}
				return m, nil
			}

			if m.historyIdx != -1 {
				if m.historyIdx < len(m.inputHistory)-1 {
					m.historyIdx++
					m.textInput.SetValue(m.inputHistory[m.historyIdx])
				} else {
					m.historyIdx = -1
					m.textInput.SetValue("")
				}
				m.textInput.CursorEnd()
				return m, nil
			}

		case tea.KeyEsc:
			if len(m.suggestions) > 0 {
				m.suggestions = nil
				m.suggestionIdx = -1
				return m, nil
			}
			if m.isRecording && m.recordCmd != nil {
				_ = m.recordCmd.Process.Kill()
			}
			return m, tea.Quit

		case tea.KeySpace:

			if m.textInput.Value() == "" {

				if m.isRecording {
					m.isRecording = false
					if m.recordCmd != nil && m.recordCmd.Process != nil {
						_ = m.recordCmd.Process.Signal(os.Interrupt)
						time.Sleep(100 * time.Millisecond)
					}

					audioBytes, err := os.ReadFile("/tmp/Mana_voice.wav")
					if err != nil {
						m.messages = append(m.messages, errStyle.Render("Error reading audio: "+err.Error()))
						return m, nil
					}

					encodedAudio := base64.StdEncoding.EncodeToString(audioBytes)

					m.awaitingVoiceChunk = true
					m.isLoading = true
					m.generatedTokens = 0
					m.generationTime = 0
					m.tokensPerSecond = 0.0

					m.activeStreams = make(map[string]string)
					m.streamOrder = nil

					req := ChatRequest{
						SessionID: m.sessionID,
						AudioData: encodedAudio,
						Agents:    m.activeAgents,
					}

					sendCmd := func() tea.Msg {
						err := m.conn.WriteJSON(req)
						if err != nil {
							m.connected = false
							return err
						}
						return nil
					}
					return m, tea.Batch(sendCmd, m.spinner.Tick)
				}

				if m.isSpeaking {
					stopSpeechCmd := func() tea.Msg {
						req := ChatRequest{
							SessionID: m.sessionID,
							Action:    "stop_speech",
						}
						_ = m.conn.WriteJSON(req)
						return nil
					}
					stopSpeechCmd()
					m.isSpeaking = false
				}

				m.isRecording = true
				m.recordCmd = exec.Command("arecord", "-f", "cd", "/tmp/Mana_voice.wav")
				if err := m.recordCmd.Start(); err != nil {
					m.messages = append(m.messages, errStyle.Render("Error starting recording: "+err.Error()))
					m.isRecording = false
					return m, nil
				}
				return m, m.spinner.Tick
			}

		case tea.KeyEnter:

			if len(m.suggestions) > 0 && m.suggestionIdx >= 0 {

				m.textInput.SetValue(applySuggestion(m.textInput.Value(), m.suggestions[m.suggestionIdx]))
				m.textInput.CursorEnd()
				m.suggestions = nil
				m.suggestionIdx = -1
				return m, nil
			}
			m.suggestions = nil
			m.suggestionIdx = -1

			input := m.textInput.Value()
			if input == "" {
				return m, nil
			}

			if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != input {
				m.inputHistory = append(m.inputHistory, input)
			}
			m.historyIdx = -1

			if input == "/help" {
				m.showHelp = true
				m.textInput.SetValue("")
				return m, nil
			}

			if strings.HasPrefix(input, "/attach") {
				m.textInput.SetValue("")
				rawPath := strings.TrimSpace(strings.TrimPrefix(input, "/attach"))

				cmdLabel := commandStyle.Render("You:")
				cmdDisplay := userBoxStyle.Width(m.width - 6).Render(
					fmt.Sprintf("%s %s", cmdLabel, input),
				)
				m.messages = append(m.messages, cmdDisplay)
				m.messageCount++

				if rawPath == "" {
					msg := "Usage: `/attach <path>`  —  supported: `.txt`, `.pdf`"
					m.messages = append(m.messages, aiBoxStyle.Width(m.width-6).Render(
						aiStyle.Render("Mana:")+"\n"+m.renderMarkdown(msg),
					))
					m.messageCount++
					m.viewport.SetContent(strings.Join(m.messages, "\n"))
					m.viewport.GotoBottom()
					return m, nil
				}

				af, err := loadAttachedFile(rawPath)
				if err != nil {
					errBox := aiBoxStyle.BorderForeground(lipgloss.Color("9")).Width(m.width - 6).Render(
						aiStyle.Render("Mana:") + "\n" + errStyle.Render("❌ "+err.Error()),
					)
					m.messages = append(m.messages, errBox)
					m.messageCount++
					m.viewport.SetContent(strings.Join(m.messages, "\n"))
					m.viewport.GotoBottom()
					return m, nil
				}

				for _, existing := range m.attachedFiles {
					if existing.Path == af.Path {
						m.notification = "⚠ Already attached: " + af.Name
						return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
							return clearNotificationMsg{}
						})
					}
				}

				m.attachedFiles = append(m.attachedFiles, af)

				sizeStr := fmt.Sprintf("%.1f KB", float64(af.Size)/1024)
				if af.Size >= 1024*1024 {
					sizeStr = fmt.Sprintf("%.1f MB", float64(af.Size)/1024/1024)
				}
				okMsg := fmt.Sprintf("📎 Attached **%s** (%s)  —  will be sent with your next message.", af.Name, sizeStr)
				m.messages = append(m.messages, aiBoxStyle.Width(m.width-6).Render(
					aiStyle.Render("Mana:")+"\n"+m.renderMarkdown(okMsg),
				))
				m.messageCount++
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
				return m, nil
			}

			if input == "/detach" {
				m.textInput.SetValue("")
				cmdDisplay := userBoxStyle.Width(m.width - 6).Render(
					fmt.Sprintf("%s %s", commandStyle.Render("You:"), input),
				)
				m.messages = append(m.messages, cmdDisplay)
				m.messageCount++

				if len(m.attachedFiles) == 0 {
					m.messages = append(m.messages, aiBoxStyle.Width(m.width-6).Render(
						aiStyle.Render("Mana:")+"\n"+infoStyle.Render("No files are currently attached."),
					))
				} else {
					m.attachedFiles = nil
					m.messages = append(m.messages, aiBoxStyle.Width(m.width-6).Render(
						aiStyle.Render("Mana:")+"\n"+m.renderMarkdown("🗑 All attachments cleared."),
					))
				}
				m.messageCount++
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
				return m, nil
			}

			if result, handled := commands.Dispatch(input, m.sessionID, m.messages); handled {
				m.textInput.SetValue("")

				cmdLabel := commandStyle.Render("You:")
				cmdDisplay := userBoxStyle.Width(m.width - 6).Render(
					fmt.Sprintf("%s %s", cmdLabel, input),
				)
				m.messages = append(m.messages, cmdDisplay)
				m.messageCount++

				if result.ViewportMessage != "" {
					rendered := m.renderMarkdown(result.ViewportMessage)
					style := aiBoxStyle
					if result.IsError {
						style = aiBoxStyle.BorderForeground(lipgloss.Color("9"))
					}
					responseBox := style.Width(m.width - 6).Render(
						aiStyle.Render("Mana:") + "\n" + rendered,
					)
					m.messages = append(m.messages, responseBox)
					m.messageCount++
				}

				if result.NewSessionID != "" {
					m.sessionID = result.NewSessionID
				}

				if result.RestoredMessages != nil {
					m.messages = result.RestoredMessages
					m.messageCount = len(result.RestoredMessages)
				}

				if result.ActiveAgents != nil {
					m.activeAgents = result.ActiveAgents
				}

				var notifCmd tea.Cmd
				if result.Notification != "" {
					m.notification = result.Notification
					notifCmd = tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
						return clearNotificationMsg{}
					})
				}

				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
				return m, notifCmd
			}

			if strings.HasPrefix(input, "/wake") {
				m.textInput.SetValue("")

				cmdLabel := senderStyle.Render("You:")
				cmdDisplay := userBoxStyle.Width(m.width - 6).Render(
					fmt.Sprintf("%s %s", cmdLabel, input),
				)
				m.messages = append(m.messages, cmdDisplay)
				m.messageCount++

				parts := strings.Fields(input)
				var agentsToWake []string
				if len(parts) > 1 {
					agentsToWake = parts[1:]
				} else {
					agentsToWake = []string{"all"}
				}

				// Resolve activeAgents based on wake target:
				//   /wake <agent>  → make that agent active
				//   /wake all      → make all agents active
				//   /wake a1 a2    → leave activeAgents alone, let user choose
				isAll := len(agentsToWake) == 1 && strings.ToLower(agentsToWake[0]) == "all"
				isSingle := len(agentsToWake) == 1 && !isAll

				if isAll {
					all := make([]string, 0, len(commands.KnownAgents))
					for slug := range commands.KnownAgents {
						all = append(all, slug)
					}
					m.activeAgents = all
				} else if isSingle {
					slug := strings.ToLower(agentsToWake[0])
					if _, ok := commands.KnownAgents[slug]; ok {
						m.activeAgents = []string{slug}
					}
				}

				req := ChatRequest{
					SessionID: m.sessionID,
					Action:    "wake_agent",
					Agents:    agentsToWake,
				}

				m.isLoading = true
				m.activeStreams = make(map[string]string)
				m.streamOrder = nil
				m.generatedTokens = 0
				m.generationTime = 0
				m.tokensPerSecond = 0.0

				content := strings.Join(m.messages, "\n")
				content += "\n" + aiStyle.Render("Mana:") + " " + m.spinner.View()
				m.viewport.SetContent(content)
				m.viewport.GotoBottom()

				sendCmd := func() tea.Msg {
					err := m.conn.WriteJSON(req)
					if err != nil {
						m.connected = false
						return err
					}
					return nil
				}
				return m, tea.Batch(sendCmd, m.spinner.Tick)
			}

			if strings.HasPrefix(input, "/online") {
				m.textInput.SetValue("")

				cmdLabel := senderStyle.Render("You:")
				cmdDisplay := userBoxStyle.Width(m.width - 6).Render(
					fmt.Sprintf("%s %s", cmdLabel, input),
				)
				m.messages = append(m.messages, cmdDisplay)
				m.messageCount++

				parts := strings.Fields(input)
				targetAgent := "all"
				if len(parts) > 1 {
					targetAgent = parts[1]
				}

				req := ChatRequest{
					SessionID: m.sessionID,
					Action:    "check_online",
					Agents:    []string{targetAgent},
				}

				m.isLoading = true
				m.activeStreams = make(map[string]string)
				m.streamOrder = nil
				m.generatedTokens = 0
				m.generationTime = 0
				m.tokensPerSecond = 0.0

				content := strings.Join(m.messages, "\n")
				content += "\n" + aiStyle.Render("Mana:") + " " + m.spinner.View()
				m.viewport.SetContent(content)
				m.viewport.GotoBottom()

				sendCmd := func() tea.Msg {
					err := m.conn.WriteJSON(req)
					if err != nil {
						m.connected = false
						return err
					}
					return nil
				}
				return m, tea.Batch(sendCmd, m.spinner.Tick)
			}

			if m.isSpeaking {
				stopSpeechCmd := func() tea.Msg {
					req := ChatRequest{
						SessionID: m.sessionID,
						Action:    "stop_speech",
					}
					_ = m.conn.WriteJSON(req)
					return nil
				}
				stopSpeechCmd()
				m.isSpeaking = false
			}

			label := senderStyle.Render("You:")
			userBubbleText := input
			if len(m.attachedFiles) > 0 {
				var names []string
				for _, af := range m.attachedFiles {
					names = append(names, "📎 "+af.Name)
				}
				userBubbleText += "\n" + infoStyle.Render(strings.Join(names, "  "))
			}
			displayInput := userBoxStyle.Width(m.width - 6).Render(
				fmt.Sprintf("%s %s", label, userBubbleText),
			)

			m.messages = append(m.messages, displayInput)
			m.messageCount++

			m.activeStreams = make(map[string]string)
			m.streamOrder = nil
			m.isLoading = true

			m.generatedTokens = 0
			m.generationTime = 0
			m.tokensPerSecond = 0.0

			content := strings.Join(m.messages, "\n")
			header := m.getAiHeader()
			content += "\n" + header + " " + m.spinner.View()

			m.viewport.SetContent(content)
			m.viewport.GotoBottom()

			var wireFiles []FileAttachment
			for _, af := range m.attachedFiles {
				wireFiles = append(wireFiles, FileAttachment{
					Name:     af.Name,
					Content:  af.Content,
					MimeType: af.MimeType,
				})
			}
			m.attachedFiles = nil

			allRe := regexp.MustCompile(`@all\b`)
			if allRe.MatchString(strings.ToLower(input)) {
				all := make([]string, 0, len(commands.KnownAgents))
				for slug := range commands.KnownAgents {
					all = append(all, slug)
				}
				m.activeAgents = all
			} else if mentioned := extractMentionedAgents(input); len(mentioned) == 1 {
				m.activeAgents = mentioned
			}

			req := ChatRequest{
				Message:         input,
				SessionID:       m.sessionID,
				SearchKnowledge: false,
				Files:           wireFiles,
				Agents:          m.activeAgents,
			}

			sendCmd := func() tea.Msg {
				err := m.conn.WriteJSON(req)
				if err != nil {
					m.connected = false
					return err
				}
				return nil
			}

			m.textInput.SetValue("")
			return m, tea.Batch(sendCmd, m.spinner.Tick)
		}

	case clearNotificationMsg:
		m.notification = ""

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 5
		m.textInput.Width = msg.Width

		coloredLogo := aiStyle.Render(manaLogo)
		centeredLogo := lipgloss.PlaceHorizontal(msg.Width, lipgloss.Center, coloredLogo)

		if len(m.messages) > 0 {
			m.messages[0] = centeredLogo
		}

		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(msg.Width-8),
		)

		content := strings.Join(m.messages, "\n")
		if len(m.streamOrder) > 0 {
			for _, agent := range m.streamOrder {
				renderedChunk := m.renderMarkdown(m.activeStreams[agent])
				fullContent := aiStyle.Render(agent+":") + "\n" + renderedChunk
				boxedContent := aiBoxStyle.Width(m.width - 6).Render(fullContent)
				content += "\n" + boxedContent
			}
		} else if m.isLoading {
			header := m.getAiHeader()
			content += "\n" + header + " " + m.spinner.View()
		}
		m.viewport.SetContent(content)

	case spinner.TickMsg:
		if m.isLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)

			if len(m.streamOrder) == 0 {
				content := strings.Join(m.messages, "\n")
				header := m.getAiHeader()
				content += "\n" + header + " " + m.spinner.View()
				m.viewport.SetContent(content)
				m.viewport.GotoBottom()
			}

			return m, cmd
		} else if m.isRecording {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case WSMessage:
		switch msg.Type {
		case "start":
			m.isLoading = true
			m.generationStart = time.Now()
			m.generatedTokens = 0
			m.activeStreams = make(map[string]string)
			m.streamOrder = nil

		case "chunk":
			if m.awaitingVoiceChunk && strings.Contains(msg.Content, "🎤 *Voice:*") {

				voiceText := strings.TrimPrefix(msg.Content, "🎤 *Voice:* ")
				voiceText = strings.TrimSpace(voiceText)

				formattedVoice := fmt.Sprintf("%s %s", senderStyle.Render("You:"), voiceText)
				displayInput := userBoxStyle.Width(m.width - 6).Render(formattedVoice)

				m.messages = append(m.messages, displayInput)
				m.messageCount++
				m.pendingVoiceInput = voiceText
				m.awaitingVoiceChunk = false

				content := strings.Join(m.messages, "\n")
				header := m.getAiHeader()
				content += "\n" + header + " " + m.spinner.View()
				m.viewport.SetContent(content)
				m.viewport.GotoBottom()
			} else {

				agentName := msg.AgentName
				if agentName == "" {
					if len(m.activeAgents) == 1 {
						slug := m.activeAgents[0]
						agentName = slug
						if n, ok := commands.KnownAgents[slug]; ok {
							agentName = n
						} else {
							agentName = strings.ToUpper(agentName[:1]) + strings.ToLower(agentName[1:])
						}
					} else {
						agentName = "Mana"
					}
				}

				if _, exists := m.activeStreams[agentName]; !exists {
					m.activeStreams[agentName] = ""
					m.streamOrder = append(m.streamOrder, agentName)
				}

				m.activeStreams[agentName] += msg.Content

				totalLen := 0
				for _, chunk := range m.activeStreams {
					totalLen += len(chunk)
				}
				m.generatedTokens = totalLen / 4

				elapsed := time.Since(m.generationStart).Seconds()
				if elapsed > 0 {
					m.tokensPerSecond = float64(m.generatedTokens) / elapsed
				}

				displayMessages := append([]string{}, m.messages...)

				for _, agent := range m.streamOrder {
					renderedContent := m.renderMarkdown(m.activeStreams[agent])
					fullContent := aiStyle.Render(agent+":") + "\n" + renderedContent
					boxedContent := aiBoxStyle.Width(m.width - 6).Render(fullContent)
					displayMessages = append(displayMessages, boxedContent)
				}

				m.viewport.SetContent(strings.Join(displayMessages, "\n"))
				m.viewport.GotoBottom()
			}

		case "end":
			if msg.GenerationTime > 0 {
				m.generationTime = time.Duration(msg.GenerationTime * float64(time.Second))
			} else {
				m.generationTime = time.Since(m.generationStart)
			}

			if msg.TokenCount > 0 {
				m.generatedTokens = msg.TokenCount
			} else {
				totalLen := 0
				for _, chunk := range m.activeStreams {
					totalLen += len(chunk)
				}
				m.generatedTokens = totalLen / 4
			}

			if m.generationTime.Seconds() > 0 {
				m.tokensPerSecond = float64(m.generatedTokens) / m.generationTime.Seconds()
			}

			m.lastAiResponse = ""

			for _, agent := range m.streamOrder {
				chunk := m.activeStreams[agent]
				m.lastAiResponse += chunk + "\n\n"

				renderedContent := m.renderMarkdown(chunk)
				fullContent := aiStyle.Render(agent+":") + "\n" + renderedContent
				boxedContent := aiBoxStyle.Width(m.width - 6).Render(fullContent)

				m.messages = append(m.messages, boxedContent)
				m.messageCount++
			}

			m.activeStreams = make(map[string]string)
			m.streamOrder = nil
			m.isLoading = false
			m.pendingVoiceInput = ""
			m.isSpeaking = true

		case "speech_stopped":
			m.isSpeaking = false

		case "error":
			errMsg := errStyle.Render("Error: " + msg.Message)
			m.messages = append(m.messages, errMsg)
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			m.isLoading = false
			m.isSpeaking = false

		default:
			if msg.Error != "" {
				m.messages = append(m.messages, errStyle.Render("System Error: "+msg.Error))
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
				m.isLoading = false
				m.connected = false
				m.isSpeaking = false
			}
		}

		return m, waitForIncomingMessage(m.conn)

	case error:
		m.err = msg
		m.connected = false
		m.isSpeaking = false
		return m, nil
	}

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	newMatches := getMatches(m.textInput.Value())
	changed := len(newMatches) != len(m.suggestions)
	if !changed {
		for i := range newMatches {
			if newMatches[i] != m.suggestions[i] {
				changed = true
				break
			}
		}
	}
	if changed {
		m.suggestionIdx = -1
	}
	m.suggestions = newMatches

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) renderFileBar() string {
	if len(m.attachedFiles) == 0 {
		return ""
	}
	var badges []string
	for _, af := range m.attachedFiles {
		icon := "📄"
		if af.MimeType == "application/pdf" {
			icon = "📕"
		}
		badges = append(badges, fileBadgeStyle.Render(icon+" "+af.Name))
	}
	hint := fileBarStyle.Render("ctrl+d to clear")
	return strings.Join(badges, " ") + "  " + hint + "\n"
}

func (m model) View() string {
	if m.err != nil {
		return errStyle.Render(fmt.Sprintf("\nFatal Error: %v\nRestart the application.", m.err))
	}

	if m.showFilePicker {
		return m.renderFilePicker()
	}

	if m.showHelp {
		title := helpTitleStyle.Render("Commands / Short Cuts :")

		content := fmt.Sprintf(`
%s

%s
  /attach <path>          Attach a .txt or .pdf file to next message
  /detach                 Remove all staged attachments
  /save-session [name]    Save current conversation
  /resume-session [name]  Restore a saved conversation
  /resume-session         List all saved sessions
  /talk <agent>           Route messages to one agent  (e.g. airi, zephyr)
  /talk <a1> <a2>         Route to multiple agents simultaneously
  /talk all               Broadcast to all agents
  /online                 Show which agents are currently online
  /online <agent>
  /wake <agent>           Start an agent's server process
  /wake <a1> <a2>         Start multiple agents in parallel
  /wake all               Start all agents
  /help                   Show this screen

%s
  ctrl + f   Open file picker (browse & attach)
  ctrl + y   Copy last Mana response to clipboard
  ctrl + d   Clear all staged file attachments
  ctrl + c   Quit

%s
`, title,
			commandStyle.Render("Slash Commands:"),
			commandStyle.Render("Shortcuts:"),
			infoStyle.Render("Press ctrl + x to close"))

		helpBox := helpBoxStyle.Render(content)

		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			helpBox,
		)
	}

	dropdownBlock := ""
	if len(m.suggestions) > 0 {
		var rows []string
		for i, cmd := range m.suggestions {
			if i == m.suggestionIdx {
				rows = append(rows, acSelectedStyle.Render(cmd))
			} else {
				rows = append(rows, acItemStyle.Render(cmd))
			}
		}
		inner := strings.Join(rows, "\n")
		dropdownBlock = acBorderStyle.Render(inner) + "\n"
	}

	return fmt.Sprintf(
		"%s\n%s\n%s%s%s",
		m.viewport.View(),
		m.renderStatusBar(),
		dropdownBlock,
		m.renderFileBar(),
		m.textInput.View(),
	) + "\n"
}

func waitForIncomingMessage(conn *websocket.Conn) tea.Cmd {
	return func() tea.Msg {
		_, bytes, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var msg WSMessage
		if err := json.Unmarshal(bytes, &msg); err != nil {
			return WSMessage{Type: "error", Message: "Invalid JSON"}
		}
		return msg
	}
}

func main() {
	port := flag.String("port", "8080", "WebSocket server port")
	flag.Parse()

	u := url.URL{
		Scheme: "ws",
		Host:   "localhost:" + *port,
		Path:   "/ws/chat",
	}

	fmt.Println("Connecting to", u.String(), "...")

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Could not connect. Is the backend running?\nError:", err)
	}
	defer conn.Close()

	p := tea.NewProgram(initialModel(conn), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal("Error running program:", err)
	}
}
