package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

type TUIHostStatus struct {
	Online  bool
	Latency time.Duration
	Checked bool
}

type CtopTUI struct {
	Config        *SSHConfig
	SelectedIndex int
	StatusMap     map[string]TUIHostStatus
	StatusMu      sync.RWMutex
	ModalOpen     bool
	ModalIndex    int
	Running       bool
	InSubprocess  bool
	SubprocessMu  sync.Mutex
	TermWidth     int
	TermHeight    int
}

func NewCtopTUI(cfg *SSHConfig) *CtopTUI {
	return &CtopTUI{
		Config:        cfg,
		SelectedIndex: 0,
		StatusMap:     make(map[string]TUIHostStatus),
		ModalOpen:     false,
		ModalIndex:    0,
		Running:       true,
		InSubprocess:  false,
		TermWidth:     80,
		TermHeight:    24,
	}
}

func (t *CtopTUI) RefreshPings() {
	var wg sync.WaitGroup
	for _, e := range t.Config.Entries {
		if e.HostName == "" || strings.Contains(e.HostName, "*") {
			continue
		}
		wg.Add(1)
		go func(alias, host, port string) {
			defer wg.Done()
			online, latency := PingHost(host, port, 1200*time.Millisecond)
			t.StatusMu.Lock()
			t.StatusMap[alias] = TUIHostStatus{
				Online:  online,
				Latency: latency,
				Checked: true,
			}
			t.StatusMu.Unlock()
		}(e.Alias, e.HostName, e.Port)
	}
}

func (t *CtopTUI) Run() error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		cmdList(t.Config, false)
		return nil
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		cmdList(t.Config, false)
		return nil
	}

	// Hide cursor on enter, restore on exit
	fmt.Print("\033[?25l")
	defer func() {
		fmt.Print("\033[?25h")
		_ = term.Restore(fd, oldState)
	}()

	// Initial async ping
	t.RefreshPings()

	// Redraw ticker for ping updates (only when not running a subprocess/edit prompt)
	go func() {
		for t.Running {
			time.Sleep(300 * time.Millisecond)
			if t.Running {
				t.SubprocessMu.Lock()
				inSub := t.InSubprocess
				t.SubprocessMu.Unlock()
				if !inSub {
					t.Draw()
				}
			}
		}
	}()

	t.Draw()

	buf := make([]byte, 16)
	for t.Running {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}

		bytes := buf[:n]

		// Handle escape sequences & keystrokes
		if bytes[0] == 3 { // Ctrl+C
			break
		}

		if t.ModalOpen {
			t.handleModalKey(bytes, fd, oldState)
		} else {
			t.handleMainKey(bytes, fd, oldState)
		}

		t.Draw()
	}

	// Clear screen before exit
	fmt.Print("\033[2J\033[H")
	return nil
}

func (t *CtopTUI) handleMainKey(bytes []byte, fd int, oldState *term.State) {
	n := len(bytes)
	totalHosts := len(t.Config.Entries)

	// Arrow keys: \x1b[A (Up), \x1b[B (Down)
	if n >= 3 && bytes[0] == 0x1b && bytes[1] == '[' {
		switch bytes[2] {
		case 'A': // Up
			if t.SelectedIndex > 0 {
				t.SelectedIndex--
			}
			return
		case 'B': // Down
			if t.SelectedIndex < totalHosts-1 {
				t.SelectedIndex++
			}
			return
		case 'H': // Home
			t.SelectedIndex = 0
			return
		case 'F': // End
			if totalHosts > 0 {
				t.SelectedIndex = totalHosts - 1
			}
			return
		}
	}

	// Single key handlers
	switch bytes[0] {
	case 'q', 'Q':
		t.Running = false
	case 'k', 'K': // Vi Up
		if t.SelectedIndex > 0 {
			t.SelectedIndex--
		}
	case 'j', 'J': // Vi Down
		if t.SelectedIndex < totalHosts-1 {
			t.SelectedIndex++
		}
	case '\r', '\n': // Enter -> open action modal
		if totalHosts > 0 {
			t.ModalOpen = true
			t.ModalIndex = 0
		}
	case 'c', 'C': // Direct connect
		if totalHosts > 0 {
			t.execConnect(t.Config.Entries[t.SelectedIndex].Alias, fd, oldState)
		}
	case 'l', 'L': // Direct edit label
		if totalHosts > 0 {
			t.execEditLabel(t.Config.Entries[t.SelectedIndex].Alias, fd, oldState)
		}
	case 'e', 'E': // Direct edit all
		if totalHosts > 0 {
			t.execEdit(t.Config.Entries[t.SelectedIndex].Alias, fd, oldState)
		}
	case 'd', 'D': // Direct delete
		if totalHosts > 0 {
			t.execDelete(t.Config.Entries[t.SelectedIndex].Alias, fd, oldState)
		}
	case 'a', 'A': // Add new host
		t.execAdd(fd, oldState)
	case 'r', 'R': // Refresh ping
		t.RefreshPings()
	case 't', 'T': // Test connection
		if totalHosts > 0 {
			t.execTest(t.Config.Entries[t.SelectedIndex].Alias, fd, oldState)
		}
	case 'p', 'P': // Ping single host
		if totalHosts > 0 {
			t.execPing(t.Config.Entries[t.SelectedIndex].Alias, fd, oldState)
		}
	case 0x1b: // Esc
		t.Running = false
	}
}

func (t *CtopTUI) handleModalKey(bytes []byte, fd int, oldState *term.State) {
	n := len(bytes)
	menuOptions := 7 // Connect, Edit Label, Edit All, CopyKey, Test, Delete, Cancel

	if n >= 3 && bytes[0] == 0x1b && bytes[1] == '[' {
		switch bytes[2] {
		case 'A': // Up
			if t.ModalIndex > 0 {
				t.ModalIndex--
			} else {
				t.ModalIndex = menuOptions - 1
			}
			return
		case 'B': // Down
			if t.ModalIndex < menuOptions-1 {
				t.ModalIndex++
			} else {
				t.ModalIndex = 0
			}
			return
		}
	}

	selectedHost := t.Config.Entries[t.SelectedIndex].Alias

	switch bytes[0] {
	case 'k', 'K':
		if t.ModalIndex > 0 {
			t.ModalIndex--
		} else {
			t.ModalIndex = menuOptions - 1
		}
	case 'j', 'J':
		if t.ModalIndex < menuOptions-1 {
			t.ModalIndex++
		} else {
			t.ModalIndex = 0
		}
	case '\r', '\n': // Select current modal option
		t.ModalOpen = false
		switch t.ModalIndex {
		case 0: // Connect
			t.execConnect(selectedHost, fd, oldState)
		case 1: // Edit Label
			t.execEditLabel(selectedHost, fd, oldState)
		case 2: // Edit All
			t.execEdit(selectedHost, fd, oldState)
		case 3: // Copy Key
			t.execCopyKey(selectedHost, fd, oldState)
		case 4: // Test
			t.execTest(selectedHost, fd, oldState)
		case 5: // Delete
			t.execDelete(selectedHost, fd, oldState)
		case 6: // Cancel
			// Do nothing
		}
	case 'c', 'C':
		t.ModalOpen = false
		t.execConnect(selectedHost, fd, oldState)
	case 'l', 'L':
		t.ModalOpen = false
		t.execEditLabel(selectedHost, fd, oldState)
	case 'e', 'E':
		t.ModalOpen = false
		t.execEdit(selectedHost, fd, oldState)
	case 'd', 'D':
		t.ModalOpen = false
		t.execDelete(selectedHost, fd, oldState)
	case '0', 'q', 'Q', 0x1b: // Esc or Q
		t.ModalOpen = false
	}
}

// Temporary restore to cooked mode for terminal interactivity
func (t *CtopTUI) restoreCooked(fd int, oldState *term.State) {
	t.SubprocessMu.Lock()
	t.InSubprocess = true
	t.SubprocessMu.Unlock()
	fmt.Print("\033[?25h\033[2J\033[H")
	_ = term.Restore(fd, oldState)
}

func (t *CtopTUI) resumeRaw(fd int) *term.State {
	newState, _ := term.MakeRaw(fd)
	fmt.Print("\033[?25l")
	// Reload config in case of edits
	newCfg, err := LoadSSHConfig()
	if err == nil {
		t.Config = newCfg
		if t.SelectedIndex >= len(t.Config.Entries) {
			t.SelectedIndex = len(t.Config.Entries) - 1
		}
		if t.SelectedIndex < 0 {
			t.SelectedIndex = 0
		}
	}
	t.SubprocessMu.Lock()
	t.InSubprocess = false
	t.SubprocessMu.Unlock()
	t.Draw()
	return newState
}

func (t *CtopTUI) execEditLabel(alias string, fd int, oldState *term.State) {
	t.restoreCooked(fd, oldState)
	fmt.Print("\033[H\033[2J")
	reader := bufio.NewReader(os.Stdin)
	cmdEditLabel(t.Config, reader, alias)
	fmt.Printf("\n%sPress Enter to return to HSSH...%s", ColorGray, ColorReset)
	Prompt(reader, "", "")
	_ = t.resumeRaw(fd)
}

func (t *CtopTUI) execConnect(alias string, fd int, oldState *term.State) {
	t.restoreCooked(fd, oldState)
	ConnectSSH(alias)
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n%sDisconnected from %s. Press Enter to return to HSSH...%s", ColorGray, alias, ColorReset)
	Prompt(reader, "", "")
	_ = t.resumeRaw(fd)
}

func (t *CtopTUI) execEdit(alias string, fd int, oldState *term.State) {
	t.restoreCooked(fd, oldState)
	fmt.Print("\033[H\033[2J")
	reader := bufio.NewReader(os.Stdin)
	cmdEdit(t.Config, reader, alias)
	fmt.Printf("\n%sPress Enter to return to HSSH...%s", ColorGray, ColorReset)
	Prompt(reader, "", "")
	_ = t.resumeRaw(fd)
}

func (t *CtopTUI) execDelete(alias string, fd int, oldState *term.State) {
	t.restoreCooked(fd, oldState)
	fmt.Print("\033[H\033[2J")
	reader := bufio.NewReader(os.Stdin)
	cmdRemove(t.Config, reader, alias)
	fmt.Printf("\n%sPress Enter to return to HSSH...%s", ColorGray, ColorReset)
	Prompt(reader, "", "")
	_ = t.resumeRaw(fd)
}

func (t *CtopTUI) execAdd(fd int, oldState *term.State) {
	t.restoreCooked(fd, oldState)
	fmt.Print("\033[H\033[2J")
	reader := bufio.NewReader(os.Stdin)
	cmdAdd(t.Config, reader, nil)
	fmt.Printf("\n%sPress Enter to return to HSSH...%s", ColorGray, ColorReset)
	Prompt(reader, "", "")
	_ = t.resumeRaw(fd)
}

func (t *CtopTUI) execCopyKey(alias string, fd int, oldState *term.State) {
	t.restoreCooked(fd, oldState)
	reader := bufio.NewReader(os.Stdin)
	cmdCopyKey(t.Config, reader, alias)
	fmt.Printf("\n%sPress Enter to return to HSSH...%s", ColorGray, ColorReset)
	Prompt(reader, "", "")
	_ = t.resumeRaw(fd)
}

func (t *CtopTUI) execTest(alias string, fd int, oldState *term.State) {
	t.restoreCooked(fd, oldState)
	reader := bufio.NewReader(os.Stdin)
	cmdTest(t.Config, reader, alias)
	fmt.Printf("\n%sPress Enter to return to HSSH...%s", ColorGray, ColorReset)
	Prompt(reader, "", "")
	_ = t.resumeRaw(fd)
}

func (t *CtopTUI) execPing(alias string, fd int, oldState *term.State) {
	t.restoreCooked(fd, oldState)
	cmdPing(t.Config, alias)
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n%sPress Enter to return to HSSH...%s", ColorGray, ColorReset)
	Prompt(reader, "", "")
	_ = t.resumeRaw(fd)
}

func (t *CtopTUI) Draw() {
	t.SubprocessMu.Lock()
	inSub := t.InSubprocess
	t.SubprocessMu.Unlock()
	if inSub {
		return
	}

	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil && width > 0 && height > 0 {
		t.TermWidth = width
		t.TermHeight = height
	}

	var b strings.Builder
	// Move cursor to top-left without clearing screen to prevent flicker
	b.WriteString("\033[H")

	// Header Bar (ctop style)
	totalHosts := len(t.Config.Entries)
	onlineCount := 0
	downCount := 0

	t.StatusMu.RLock()
	for _, status := range t.StatusMap {
		if status.Checked {
			if status.Online {
				onlineCount++
			} else {
				downCount++
			}
		}
	}
	t.StatusMu.RUnlock()

	headerLeft := fmt.Sprintf(" %s%sHSSH%s • %d hosts (%s● %d online%s, %s○ %d down%s)",
		ColorBold, ColorCyan, ColorReset,
		totalHosts,
		ColorGreen, onlineCount, ColorReset,
		ColorRed, downCount, ColorReset)

	headerRight := fmt.Sprintf("%s[a] Add  [r] Refresh  [q] Quit%s ", ColorGray, ColorReset)
	spacing := t.TermWidth - 70
	if spacing < 2 {
		spacing = 2
	}
	if t.TermWidth < 75 {
		headerRight = fmt.Sprintf("%s[q] Quit%s ", ColorGray, ColorReset)
		spacing = t.TermWidth - 50
		if spacing < 2 {
			spacing = 2
		}
	}
	b.WriteString(fmt.Sprintf("%s%s%s\033[K\r\n", headerLeft, strings.Repeat(" ", spacing), headerRight))

	// Separator rule
	ruleLen := t.TermWidth
	if ruleLen > 92 {
		ruleLen = 92
	}
	b.WriteString(fmt.Sprintf("%s%s%s\r\n", ColorGray, strings.Repeat("─", ruleLen), ColorReset))

	// Column Headers
	b.WriteString(fmt.Sprintf("  %s%-18s %-12s %-22s %-12s %-6s %-16s%s\r\n",
		ColorBold+ColorWhite, "ALIAS", "STATUS", "HOSTNAME / IP", "USER", "PORT", "LABEL", ColorReset))
	b.WriteString(fmt.Sprintf("%s%s%s\r\n", ColorGray, strings.Repeat("─", ruleLen), ColorReset))

	// Rows
	if totalHosts == 0 {
		b.WriteString(fmt.Sprintf("\r\n  %sNo hosts configured. Press 'a' to add a server.%s\r\n", ColorYellow, ColorReset))
	} else {
		for i, e := range t.Config.Entries {
			statusStr := fmt.Sprintf("%s- checking%s", ColorGray, ColorReset)

			t.StatusMu.RLock()
			status, exists := t.StatusMap[e.Alias]
			t.StatusMu.RUnlock()

			if exists && status.Checked {
				if status.Online {
					ms := status.Latency.Milliseconds()
					if ms < 100 {
						statusStr = fmt.Sprintf("%s● %dms%s", ColorGreen, ms, ColorReset)
					} else {
						statusStr = fmt.Sprintf("%s● %dms%s", ColorYellow, ms, ColorReset)
					}
				} else {
					statusStr = fmt.Sprintf("%s○ Down%s", ColorRed, ColorReset)
				}
			}

			user := e.User
			if user == "" {
				user = "-"
			}
			tags := e.Tags
			if tags == "" {
				tags = "-"
			}

			isSelected := (i == t.SelectedIndex)
			cursor := "  "
			rowColor := ColorReset
			aliasColor := ColorWhite

			if isSelected {
				cursor = fmt.Sprintf("%s❯%s ", ColorCyan+ColorBold, ColorReset)
				rowColor = "\033[48;5;236m" // Subtle dark gray highlight
				aliasColor = ColorCyan + ColorBold
			}

			line := fmt.Sprintf("%s%s%-18s%s %-20s %-22s %-12s %-6s %s%-16s%s",
				cursor,
				aliasColor, e.Alias, ColorReset+rowColor,
				statusStr,
				e.HostName,
				user,
				e.Port,
				ColorCyan, tags, ColorReset)

			if isSelected {
				b.WriteString(fmt.Sprintf("%s%s%s\033[K\r\n", rowColor, line, ColorReset))
			} else {
				b.WriteString(fmt.Sprintf("%s\033[K\r\n", line))
			}
		}
	}

	b.WriteString(fmt.Sprintf("%s%s%s\r\n", ColorGray, strings.Repeat("─", ruleLen), ColorReset))

	// Modal or Footer Toolbar
	if t.ModalOpen && totalHosts > 0 {
		selectedHost := t.Config.Entries[t.SelectedIndex]
		t.renderModal(&b, selectedHost)
	} else {
		if t.TermWidth < 82 {
			b.WriteString(fmt.Sprintf("  %s[Enter]%s Actions  %s[l]%s Label  %s[e]%s Edit  %s[c]%s Connect  %s[q]%s Quit\033[K\r\n",
				ColorCyan+ColorBold, ColorReset,
				ColorCyan+ColorBold, ColorReset,
				ColorCyan+ColorBold, ColorReset,
				ColorCyan+ColorBold, ColorReset,
				ColorCyan+ColorBold, ColorReset))
		} else {
			b.WriteString(fmt.Sprintf("  %s[↑/↓]%s Navigate  %s[Enter]%s Actions  %s[c]%s Connect  %s[l]%s Label  %s[e]%s Edit  %s[d]%s Delete  %s[q]%s Quit\033[K\r\n",
				ColorBold, ColorReset,
				ColorBold, ColorReset,
				ColorBold, ColorReset,
				ColorCyan+ColorBold, ColorReset,
				ColorBold, ColorReset,
				ColorBold, ColorReset,
				ColorBold, ColorReset))
		}
	}

	// Clear rest of terminal below
	b.WriteString("\033[J")

	// Flush buffer
	fmt.Print(b.String())
}

func (t *CtopTUI) renderModal(b *strings.Builder, host *HostEntry) {
	labelStr := host.Tags
	if labelStr == "" {
		labelStr = "none"
	}
	modalTitle := fmt.Sprintf(" Actions: %s [label: %s] ", host.Alias, labelStr)
	if len(modalTitle) > 52 {
		modalTitle = fmt.Sprintf(" Actions: %s ", host.Alias)
	}

	boxWidth := 58
	topBorder := fmt.Sprintf("┌──%s%s%s%s┐", ColorBold+ColorCyan, modalTitle, ColorReset, strings.Repeat("─", boxWidth-len(modalTitle)-4))
	botBorder := fmt.Sprintf("└──%s┘", strings.Repeat("─", boxWidth-4))

	b.WriteString(fmt.Sprintf("\r\n  %s\r\n", topBorder))

	options := []struct {
		key  string
		desc string
	}{
		{"c / Enter", "Connect via SSH"},
		{"l", "Edit host label / tags"},
		{"e", "Edit host settings (IP, user, port)"},
		{"k", "Copy public SSH key (passwordless setup)"},
		{"t", "Test passwordless connection"},
		{"d", "Delete host entry"},
		{"Esc", "Cancel / Return to host list"},
	}

	for i, opt := range options {
		cursor := "  "
		highlight := ColorReset
		if i == t.ModalIndex {
			cursor = fmt.Sprintf("%s❯%s ", ColorCyan+ColorBold, ColorReset)
			highlight = "\033[48;5;236m"
		}

		line := fmt.Sprintf("%s[%s%-9s%s] %s%-34s%s",
			cursor,
			ColorBold+ColorWhite, opt.key, ColorReset+highlight,
			ColorWhite, opt.desc, ColorReset)

		b.WriteString(fmt.Sprintf("  │ %s%s%s │\r\n", highlight, line, ColorReset))
	}

	b.WriteString(fmt.Sprintf("  %s\r\n", botBorder))
}
