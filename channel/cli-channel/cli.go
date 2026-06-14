package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorCyan    = "\033[36m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorMagenta = "\033[35m"
	colorGray    = "\033[90m"
	colorBold    = "\033[1m"
)

// CLI manages the interactive chat session.
type CLI struct {
	bridge     *Bridge
	cfg        *Config
	sessionKey string
	agentID    string
	history    []chatEntry // local message history for display
	ctx        context.Context
	cancel     context.CancelFunc
}

type chatEntry struct {
	role    string // "user" or "assistant"
	content string
	time    time.Time
}

// NewCLI creates a new interactive CLI.
func NewCLI(bridge *Bridge, cfg *Config) *CLI {
	ctx, cancel := context.WithCancel(context.Background())
	agentID := cfg.AgentKey
	if agentID == "" {
		agentID = "default"
	}
	return &CLI{
		bridge:     bridge,
		cfg:        cfg,
		sessionKey: fmt.Sprintf("agent:%s:cli:%s", agentID, uuid.NewString()),
		agentID:    agentID,
		history:    make([]chatEntry, 0, 100),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run starts the interactive REPL.
func (c *CLI) Run() {
	defer c.cancel()

	printCLIBanner(c.cfg)

	scanner := bufio.NewScanner(os.Stdin)
	// Increase buffer for long pasted inputs
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		fmt.Printf("%s%sYou>%s ", colorBold, colorGreen, colorReset)
		if !scanner.Scan() {
			// EOF (Ctrl+D)
			fmt.Printf("\n%sBye!%s\n", colorGray, colorReset)
			return
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if c.handleCommand(input) {
				return // /quit
			}
			continue
		}

		// Send chat message
		c.sendMessage(input)
	}

	if err := scanner.Err(); err != nil {
		slog.Error("scanner error", "error", err)
	}
}

// sendMessage sends a message and displays the streaming response.
func (c *CLI) sendMessage(message string) {
	start := time.Now()

	// Record user message
	c.history = append(c.history, chatEntry{
		role:    "user",
		content: message,
		time:    start,
	})

	// Use ChatSend (sync) for simplicity — goclaw returns the full response
	// after the agent completes. Streaming via events is complex because
	// sendRequest blocks until the response arrives.
	fmt.Printf("%s%sAssistant>%s ", colorBold, colorMagenta, colorReset)
	fmt.Print(colorGray)
	fmt.Print("thinking...")
	fmt.Print(colorReset)
	fmt.Print("\r")
	fmt.Printf("%s%sAssistant>%s ", colorBold, colorMagenta, colorReset)

	content, err := c.bridge.ChatSend(c.ctx, message, c.agentID)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("%sError: %s%s\n", colorRed, err.Error(), colorReset)
		slog.Error("chat failed", "error", err, "elapsed", elapsed)
		return
	}

	// Clear the "thinking..." line and print actual response
	// Move cursor up one line and clear it
	fmt.Printf("\033[A\033[2K")
	fmt.Printf("%s%sAssistant>%s ", colorBold, colorMagenta, colorReset)

	// Print content with word wrapping
	printWrapped(content)

	// Show timing info
	fmt.Printf("%s[%s]%s\n", colorGray, formatDuration(elapsed), colorReset)

	// Record assistant response
	c.history = append(c.history, chatEntry{
		role:    "assistant",
		content: content,
		time:    time.Now(),
	})
}

// handleCommand processes slash commands. Returns true if should exit.
func (c *CLI) handleCommand(input string) bool {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/quit", "/exit", "/q":
		fmt.Printf("%sBye!%s\n", colorGray, colorReset)
		return true

	case "/help", "/h":
		c.printHelp()

	case "/clear":
		c.history = c.history[:0]
		c.sessionKey = fmt.Sprintf("agent:%s:cli:%s", c.agentID, uuid.NewString())
		fmt.Printf("%sSession cleared. New session started.%s\n", colorYellow, colorReset)

	case "/history":
		c.printHistory()

	case "/agent":
		if len(parts) > 1 {
			c.agentID = parts[1]
			c.sessionKey = fmt.Sprintf("agent:%s:cli:%s", c.agentID, uuid.NewString())
			fmt.Printf("%sSwitched to agent: %s%s\n", colorYellow, c.agentID, colorReset)
		} else {
			fmt.Printf("%sCurrent agent: %s%s\n", colorYellow, c.agentID, colorReset)
		}

	case "/session":
		fmt.Printf("%sSession: %s%s\n", colorYellow, c.sessionKey, colorReset)

	case "/skills":
		c.showSkills()

	case "/status":
		c.showStatus()

	case "/history-server":
		c.showServerHistory()

	case "/abort":
		if err := c.bridge.Abort(c.ctx, c.sessionKey); err != nil {
			fmt.Printf("%sAbort failed: %s%s\n", colorRed, err.Error(), colorReset)
		} else {
			fmt.Printf("%sRequest aborted.%s\n", colorYellow, colorReset)
		}

	default:
		fmt.Printf("%sUnknown command: %s (type /help for available commands)%s\n", colorRed, cmd, colorReset)
	}

	return false
}

// printHelp shows available commands.
func (c *CLI) printHelp() {
	fmt.Printf(`
%s╔══════════════════════════════════════════════════════════════╗
║                    Available Commands                        ║
╚══════════════════════════════════════════════════════════════╝%s

  %sChat:%s
    <message>             Send a message to the agent
    /abort                Abort the current request

  %sSession:%s
    /clear                Clear history and start new session
    /history              Show local message history
    /history-server       Show server-side session history
    /session              Show current session key

  %sAgent:%s
    /agent                Show current agent
    /agent <name>         Switch to a different agent
    /skills               List available skills
    /status               Show bridge and connection status

  %sGeneral:%s
    /help                 Show this help
    /quit, /exit, /q      Exit the CLI

`, colorCyan, colorReset,
		colorBold, colorReset,
		colorBold, colorReset,
		colorBold, colorReset,
		colorBold, colorReset)
}

// printHistory shows local chat history.
func (c *CLI) printHistory() {
	if len(c.history) == 0 {
		fmt.Printf("%sNo messages yet.%s\n", colorGray, colorReset)
		return
	}

	fmt.Printf("\n%s── Chat History (%d messages) ──%s\n\n", colorCyan, len(c.history), colorReset)
	for i, entry := range c.history {
		role := "You"
		roleColor := colorGreen
		if entry.role == "assistant" {
			role = "Assistant"
			roleColor = colorMagenta
		}
		fmt.Printf("%s%s[%d] %s%s ", colorGray, colorReset, i+1, roleColor, role)
		// Truncate long messages for display
		content := entry.content
		if len(content) > 120 {
			content = content[:117] + "..."
		}
		// Replace newlines for single-line display
		content = strings.ReplaceAll(content, "\n", " ↵ ")
		fmt.Printf("%s%s%s\n", colorReset, content, colorReset)
	}
	fmt.Println()
}

// showSkills lists available skills from goclaw.
func (c *CLI) showSkills() {
	result, err := c.bridge.SkillsList(c.ctx)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", colorRed, err.Error(), colorReset)
		return
	}

	data, _ := json.MarshalIndent(result, "  ", "  ")
	// Try to extract skill names from the payload
	if payloadMap, ok := result.(map[string]any); ok {
		if skills, ok := payloadMap["skills"].([]any); ok {
			fmt.Printf("\n%s── Available Skills (%d) ──%s\n\n", colorCyan, len(skills), colorReset)
			for _, s := range skills {
				if sm, ok := s.(map[string]any); ok {
					name := sm["name"]
					desc := ""
					if d, ok := sm["description"].(string); ok && len(d) > 80 {
						desc = d[:77] + "..."
					} else if d, ok := sm["description"].(string); ok {
						desc = d
					}
					enabled := "✓"
					if e, ok := sm["enabled"].(bool); ok && !e {
						enabled = "✗"
					}
					fmt.Printf("  %s %s%s%s %s%s%s\n", enabled, colorBold, name, colorReset, colorGray, desc, colorReset)
				}
			}
			fmt.Println()
			return
		}
	}

	// Fallback: print raw JSON
	fmt.Printf("\n%s── Skills ──%s\n%s\n", colorCyan, colorReset, string(data))
}

// showStatus displays bridge status.
func (c *CLI) showStatus() {
	connected := c.bridge.IsConnected()
	connStr := fmt.Sprintf("%sConnected%s", colorGreen, colorReset)
	if !connected {
		connStr = fmt.Sprintf("%sDisconnected%s", colorRed, colorReset)
	}

	fmt.Printf(`
%s── Bridge Status ──%s
  Connection:  %s
  Goclaw URL:  %s
  Agent:       %s
  Session:     %s
  Messages:    %d

`, colorCyan, colorReset, connStr, c.cfg.GoclawURL, c.agentID, c.sessionKey, len(c.history))
}

// showServerHistory fetches and displays server-side history.
func (c *CLI) showServerHistory() {
	result, err := c.bridge.History(c.ctx, c.sessionKey, 50)
	if err != nil {
		fmt.Printf("%sError fetching history: %s%s\n", colorRed, err.Error(), colorReset)
		return
	}

	data, _ := json.MarshalIndent(result, "  ", "  ")
	fmt.Printf("\n%s── Server History ──%s\n%s\n", colorCyan, colorReset, string(data))
}

// printCLIBanner prints the CLI startup banner.
func printCLIBanner(cfg *Config) {
	agentID := cfg.AgentKey
	if agentID == "" {
		agentID = "default"
	}

	top := "\u2554" + strings.Repeat("\u2550", 62) + "\u2557"
	bot := "\u255a" + strings.Repeat("\u2550", 62) + "\u255d"

	fmt.Println(colorCyan + top)
	fmt.Printf("\u2551           GoClaw Interactive CLI v1.0.0                      \u2551\n")
	fmt.Printf("\u2551  Type %s/help%s for commands, %s/quit%s to exit                   \u2551\n",
		colorBold, colorReset, colorBold, colorReset)
	fmt.Println(colorCyan + bot + colorReset)
	fmt.Println()
	fmt.Printf("  %sAgent:%s   %s%s%s\n", colorBold, colorReset, colorYellow, agentID, colorReset)
	fmt.Printf("  %sGoclaw:%s  %s\n", colorBold, colorReset, cfg.GoclawURL)
	fmt.Printf("  %sSession:%s %s(persistent)%s\n\n", colorBold, colorReset, colorGray, colorReset)
}

// printWrapped prints text with basic word wrapping.
func printWrapped(text string) {
	if text == "" {
		return
	}

	// Get terminal width (fallback to 80)
	width := 80

	// Simple word wrap
	words := strings.Fields(text)
	lineLen := 0
	for _, word := range words {
		if lineLen+len(word)+1 > width && lineLen > 0 {
			fmt.Println()
			lineLen = 0
		}
		if lineLen > 0 {
			fmt.Print(" ")
			lineLen++
		}
		fmt.Print(word)
		lineLen += len(word)
	}
	if lineLen > 0 {
		fmt.Println()
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%.0fs", int(d.Minutes()), d.Seconds()-float64(int(d.Minutes()))*60)
}
