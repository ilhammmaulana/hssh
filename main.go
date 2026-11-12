package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	cfg, err := LoadSSHConfig()
	if err != nil {
		fmt.Printf("%sFailed to load SSH config: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			tui := NewCtopTUI(cfg)
			_ = tui.Run()
		} else {
			cmdList(cfg, false)
		}
		return
	}

	subcmd := strings.ToLower(args[0])
	subargs := args[1:]

	switch subcmd {
	case "list", "ls", "-l":
		// If running in an interactive terminal without --plain, launch ctop-style TUI
		if term.IsTerminal(int(os.Stdin.Fd())) && (len(subargs) == 0 || subargs[0] != "--plain") {
			tui := NewCtopTUI(cfg)
			_ = tui.Run()
		} else {
			cmdList(cfg, true)
		}
	case "list-fast", "lf":
		cmdList(cfg, false)
	case "list-plain", "lp":
		cmdList(cfg, true)
	case "menu":
		runInteractiveMenu(cfg, reader)
	case "add", "new", "-a":
		cmdAdd(cfg, reader, subargs)
	case "connect", "c":
		target := ""
		if len(subargs) > 0 {
			target = subargs[0]
		}
		cmdConnect(cfg, reader, target)
	case "edit", "update", "modify":
		target := ""
		if len(subargs) > 0 {
			target = subargs[0]
		}
		cmdEdit(cfg, reader, target)
	case "remove", "rm", "delete":
		target := ""
		if len(subargs) > 0 {
			target = subargs[0]
		}
		cmdRemove(cfg, reader, target)
	case "test", "check":
		target := ""
		if len(subargs) > 0 {
			target = subargs[0]
		}
		cmdTest(cfg, reader, target)
	case "ping":
		target := ""
		if len(subargs) > 0 {
			target = subargs[0]
		}
		cmdPing(cfg, target)
	case "copy-key", "copy":
		target := ""
		if len(subargs) > 0 {
			target = subargs[0]
		}
		cmdCopyKey(cfg, reader, target)
	case "backup":
		bak, err := cfg.Backup()
		if err != nil {
			fmt.Printf("%sBackup failed: %v%s\n", ColorRed, err, ColorReset)
		} else {
			fmt.Printf("%s✓ Safety backup created at: %s%s\n", ColorGreen, bak, ColorReset)
		}
	case "keygen":
		_, _ = EnsureDefaultSSHKey(reader)
	case "help", "--help", "-h":
		printHelp()
	default:
		// If argument directly matches an alias, connect immediately!
		if cfg.Find(subcmd) != nil {
			ConnectSSH(subcmd)
		} else {
			fmt.Printf("%sUnknown command or host: '%s'%s\n", ColorRed, subcmd, ColorReset)
			fmt.Printf("Run '%shssh help%s' for available commands.\n", ColorCyan, ColorReset)
			os.Exit(1)
		}
	}
}

func printHelp() {
	PrintBanner()
	fmt.Printf(`%sUsage:%s
  hssh                                 Launch interactive host dashboard (ctop-style)
  hssh list (or ls)                    Interactive host table with live latency ping
  hssh list-plain (or lp)              Plain text table without TUI
  hssh add [alias] [ip] [user]         Add a new server & setup passwordless key
  hssh connect [alias]                 Connect to a server (or 'hssh <alias>')
  hssh edit [alias]                    Edit an existing server config
  hssh remove [alias]                  Safely delete a server
  hssh test [alias]                    Test passwordless authentication
  hssh ping [alias]                    Check TCP reachability & latency
  hssh copy-key [alias]                Send public key to remote host
  hssh backup                          Create timestamped backup of ~/.ssh/config
  hssh keygen                          Generate modern ed25519 key pair

%sInteractive TUI Controls:%s
  [↑ / ↓] or [k / j]                   Navigate between hosts
  [Enter]                              Open Actions Modal (Connect, Edit, Delete, etc.)
  [c]                                  Connect via SSH immediately
  [e]                                  Edit configuration of selected host
  [d]                                  Delete selected host entry
  [a]                                  Add new host
  [r]                                  Refresh live ping latency
  [q] or [Esc]                         Quit
`, ColorBold, ColorReset, ColorBold, ColorReset)
}

func cmdList(cfg *SSHConfig, checkPing bool) {
	if len(cfg.Entries) == 0 {
		fmt.Printf("%sNo SSH hosts configured in %s%s\n", ColorYellow, cfg.FilePath, ColorReset)
		fmt.Printf("Use '%shssh add%s' to add your first server.\n", ColorCyan, ColorReset)
		return
	}

	type pingResult struct {
		online  bool
		latency time.Duration
	}

	results := make(map[string]pingResult)

	if checkPing {
		fmt.Printf("%sPinging servers...%s\r", ColorGray, ColorReset)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, e := range cfg.Entries {
			if e.HostName == "" || strings.Contains(e.HostName, "*") {
				continue
			}
			wg.Add(1)
			go func(host, port, alias string) {
				defer wg.Done()
				online, latency := PingHost(host, port, 1200*time.Millisecond)
				mu.Lock()
				results[alias] = pingResult{online: online, latency: latency}
				mu.Unlock()
			}(e.HostName, e.Port, e.Alias)
		}
		wg.Wait()
		fmt.Print("\r                         \r")
	}

	fmt.Printf("\n%s%sConfigured SSH Hosts:%s\n", ColorBold, ColorWhite, ColorReset)
	fmt.Printf("%s%s%s\n", ColorGray, strings.Repeat("=", 88), ColorReset)
	fmt.Printf("%s%-18s %-12s %-22s %-12s %-6s %-12s%s\n",
		ColorBold, "ALIAS", "STATUS", "HOSTNAME / IP", "USER", "PORT", "TAGS", ColorReset)
	fmt.Printf("%s%s%s\n", ColorGray, strings.Repeat("-", 88), ColorReset)

	for _, e := range cfg.Entries {
		statusStr := fmt.Sprintf("%s-%s", ColorGray, ColorReset)
		if checkPing {
			if res, ok := results[e.Alias]; ok {
				if res.online {
					ms := res.latency.Milliseconds()
					if ms < 100 {
						statusStr = fmt.Sprintf("%s● %dms%s", ColorGreen, ms, ColorReset)
					} else {
						statusStr = fmt.Sprintf("%s● %dms%s", ColorYellow, ms, ColorReset)
					}
				} else {
					statusStr = fmt.Sprintf("%s○ Down%s", ColorRed, ColorReset)
				}
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

		fmt.Printf("%s%-18s%s %-20s %-22s %-12s %-6s %s%-12s%s\n",
			ColorGreen+ColorBold, e.Alias, ColorReset,
			statusStr,
			e.HostName,
			user,
			e.Port,
			ColorCyan, tags, ColorReset)
	}
	fmt.Printf("%s%s%s\n\n", ColorGray, strings.Repeat("=", 88), ColorReset)
}

func cmdAdd(cfg *SSHConfig, reader *bufio.Reader, args []string) {
	PrintBanner()
	fmt.Printf("%s--- Add New SSH Server ---%s\n\n", ColorBold, ColorReset)

	var alias, hostname, user, port, tags string

	if len(args) >= 1 {
		alias = args[0]
	}
	if len(args) >= 2 {
		hostname = args[1]
	}
	if len(args) >= 3 {
		user = args[2]
	}
	if len(args) >= 4 {
		port = args[3]
	}

	if alias == "" {
		alias = Prompt(reader, "Server Alias / Name (e.g. prod-api, vps-sg)", "")
		for alias == "" {
			fmt.Printf("%sAlias cannot be empty!%s\n", ColorRed, ColorReset)
			alias = Prompt(reader, "Server Alias / Name", "")
		}
	}
	alias = strings.ReplaceAll(alias, " ", "-")

	if cfg.Find(alias) != nil {
		fmt.Printf("%sWarning: Host '%s' already exists in config.%s\n", ColorYellow, alias, ColorReset)
		overwrite := strings.ToLower(Prompt(reader, "Overwrite existing host? (y/N)", "N"))
		if overwrite != "y" && overwrite != "yes" {
			fmt.Println("Cancelled.")
			return
		}
	}

	if hostname == "" {
		hostname = Prompt(reader, "Server IP or Hostname (e.g. 192.168.1.50 or domain.com)", "")
		for hostname == "" {
			fmt.Printf("%sHost IP or Hostname is required!%s\n", ColorRed, ColorReset)
			hostname = Prompt(reader, "Server IP or Hostname", "")
		}
	}

	if user == "" {
		user = Prompt(reader, "Remote Username", GetCurrentUsername())
	}

	if port == "" {
		port = Prompt(reader, "SSH Port", "22")
	}

	tags = Prompt(reader, "Tags / Environment (optional, e.g. prod,api)", "")

	privKeyPath, _ := EnsureDefaultSSHKey(reader)
	identityKey := "~/.ssh/id_ed25519"
	optKey := strings.ToLower(Prompt(reader, fmt.Sprintf("Use default key (%s%s%s)? (Y/n)", ColorCyan, identityKey, ColorReset), "Y"))
	if optKey == "n" || optKey == "no" {
		identityKey = Prompt(reader, "Enter custom identity key path", identityKey)
	}

	newEntry := &HostEntry{
		Alias:         alias,
		HostName:      hostname,
		User:          user,
		Port:          port,
		IdentityFile:  identityKey,
		Tags:          tags,
		CustomOptions: make(map[string]string),
	}

	cfg.AddOrUpdate(newEntry)
	if err := cfg.Save(); err != nil {
		fmt.Printf("%sFailed to save config: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("\n%s✓ Successfully saved [%s] (%s@%s:%s) to %s!%s\n",
		ColorGreen, alias, user, hostname, port, cfg.FilePath, ColorReset)

	// Copy key prompt
	copyOpt := strings.ToLower(Prompt(reader, fmt.Sprintf("Copy public key to [%s@%s] now for passwordless login? (Y/n)", user, hostname), "Y"))
	if copyOpt == "y" || copyOpt == "yes" || copyOpt == "" {
		pubKeyPath := privKeyPath + ".pub"
		CopySSHKey(hostname, user, port, pubKeyPath, reader)
	}

	connectNow := strings.ToLower(Prompt(reader, fmt.Sprintf("Connect to [%s] right now? (y/N)", alias), "N"))
	if connectNow == "y" || connectNow == "yes" {
		ConnectSSH(alias)
	}
}

func cmdConnect(cfg *SSHConfig, reader *bufio.Reader, target string) {
	if len(cfg.Entries) == 0 {
		fmt.Printf("%sNo servers configured.%s\n", ColorYellow, ColorReset)
		cmdAdd(cfg, reader, nil)
		return
	}

	if target == "" {
		fmt.Printf("\n%sSelect a server to connect:%s\n", ColorBold, ColorReset)
		for i, e := range cfg.Entries {
			fmt.Printf("  %s[%d]%s %s%s%s (%s@%s:%s)\n",
				ColorCyan, i+1, ColorReset,
				ColorBold, e.Alias, ColorReset,
				e.User, e.HostName, e.Port)
		}
		fmt.Println()
		choice := Prompt(reader, "Enter number or alias", "")
		if num, err := strconv.Atoi(choice); err == nil && num >= 1 && num <= len(cfg.Entries) {
			target = cfg.Entries[num-1].Alias
		} else {
			target = choice
		}
	}

	if target == "" {
		return
	}

	if cfg.Find(target) == nil {
		fmt.Printf("%sHost '%s' not found in config.%s\n", ColorRed, target, ColorReset)
		return
	}

	ConnectSSH(target)
}

func cmdEdit(cfg *SSHConfig, reader *bufio.Reader, target string) {
	if len(cfg.Entries) == 0 {
		fmt.Printf("%sNo servers to edit.%s\n", ColorYellow, ColorReset)
		return
	}

	if target == "" {
		fmt.Printf("\n%sSelect a server to edit:%s\n", ColorBold, ColorReset)
		for i, e := range cfg.Entries {
			fmt.Printf("  %s[%d]%s %s (%s)\n", ColorCyan, i+1, ColorReset, e.Alias, e.HostName)
		}
		choice := Prompt(reader, "Enter number or alias", "")
		if num, err := strconv.Atoi(choice); err == nil && num >= 1 && num <= len(cfg.Entries) {
			target = cfg.Entries[num-1].Alias
		} else {
			target = choice
		}
	}

	entry := cfg.Find(target)
	if entry == nil {
		fmt.Printf("%sHost '%s' not found.%s\n", ColorRed, target, ColorReset)
		return
	}

	fmt.Printf("\n%sEditing [%s] (press Enter to keep current value):%s\n", ColorBold, entry.Alias, ColorReset)
	newAlias := Prompt(reader, "Alias", entry.Alias)
	newHost := Prompt(reader, "Hostname / IP", entry.HostName)
	newUser := Prompt(reader, "Username", entry.User)
	newPort := Prompt(reader, "Port", entry.Port)
	newKey := Prompt(reader, "Identity Key", entry.IdentityFile)
	newTags := Prompt(reader, "Tags", entry.Tags)

	if !strings.EqualFold(newAlias, entry.Alias) {
		cfg.Remove(entry.Alias)
	}

	entry.Alias = newAlias
	entry.HostName = newHost
	entry.User = newUser
	entry.Port = newPort
	entry.IdentityFile = newKey
	entry.Tags = newTags

	cfg.AddOrUpdate(entry)
	if err := cfg.Save(); err != nil {
		fmt.Printf("%sFailed to save config: %v%s\n", ColorRed, err, ColorReset)
	} else {
		fmt.Printf("%s✓ Server [%s] updated successfully!%s\n", ColorGreen, newAlias, ColorReset)
	}
}

func cmdRemove(cfg *SSHConfig, reader *bufio.Reader, target string) {
	if len(cfg.Entries) == 0 {
		fmt.Printf("%sNo servers to remove.%s\n", ColorYellow, ColorReset)
		return
	}

	if target == "" {
		fmt.Printf("\n%sSelect a server to remove:%s\n", ColorBold, ColorReset)
		for i, e := range cfg.Entries {
			fmt.Printf("  %s[%d]%s %s (%s)\n", ColorRed, i+1, ColorReset, e.Alias, e.HostName)
		}
		choice := Prompt(reader, "Enter number or alias to DELETE", "")
		if num, err := strconv.Atoi(choice); err == nil && num >= 1 && num <= len(cfg.Entries) {
			target = cfg.Entries[num-1].Alias
		} else {
			target = choice
		}
	}

	if target == "" {
		return
	}

	confirm := strings.ToLower(Prompt(reader, fmt.Sprintf("Are you sure you want to delete [%s]? (y/N)", target), "N"))
	if confirm == "y" || confirm == "yes" {
		if cfg.Remove(target) {
			_ = cfg.Save()
			fmt.Printf("%s✓ Server [%s] removed from config.%s\n", ColorGreen, target, ColorReset)
		} else {
			fmt.Printf("%sHost '%s' not found.%s\n", ColorRed, target, ColorReset)
		}
	}
}

func cmdTest(cfg *SSHConfig, reader *bufio.Reader, target string) {
	if len(cfg.Entries) == 0 {
		fmt.Printf("%sNo servers to test.%s\n", ColorYellow, ColorReset)
		return
	}

	if target == "" {
		fmt.Printf("\n%sSelect a server to test passwordless authentication:%s\n", ColorBold, ColorReset)
		for i, e := range cfg.Entries {
			fmt.Printf("  %s[%d]%s %s\n", ColorCyan, i+1, ColorReset, e.Alias)
		}
		choice := Prompt(reader, "Enter number or alias", "")
		if num, err := strconv.Atoi(choice); err == nil && num >= 1 && num <= len(cfg.Entries) {
			target = cfg.Entries[num-1].Alias
		} else {
			target = choice
		}
	}

	if target == "" {
		return
	}

	fmt.Printf("%sTesting passwordless authentication to [%s]...%s\n", ColorCyan, target, ColorReset)
	ok, errMsg := TestPasswordless(target)
	if ok {
		fmt.Printf("%s✓ SUCCESS: Passwordless connection to [%s] is working!%s\n", ColorGreen, target, ColorReset)
	} else {
		fmt.Printf("%s✗ FAILED: Unable to authenticate without password.%s\n", ColorRed, ColorReset)
		if errMsg != "" {
			fmt.Printf("%sDetails: %s%s\n", ColorGray, errMsg, ColorReset)
		}
		fix := strings.ToLower(Prompt(reader, fmt.Sprintf("Copy SSH key to [%s] now? (Y/n)", target), "Y"))
		if fix == "y" || fix == "yes" || fix == "" {
			entry := cfg.Find(target)
			if entry != nil {
				CopySSHKey(entry.HostName, entry.User, entry.Port, "", reader)
			}
		}
	}
}

func cmdPing(cfg *SSHConfig, target string) {
	if target == "" {
		cmdList(cfg, true)
		return
	}

	entry := cfg.Find(target)
	if entry == nil {
		fmt.Printf("%sHost '%s' not found.%s\n", ColorRed, target, ColorReset)
		return
	}

	fmt.Printf("Pinging %s:%s (%s)...\n", entry.HostName, entry.Port, entry.Alias)
	online, latency := PingHost(entry.HostName, entry.Port, 3*time.Second)
	if online {
		fmt.Printf("%s✓ Reachable! Latency: %v%s\n", ColorGreen, latency, ColorReset)
	} else {
		fmt.Printf("%s✗ Unreachable or port is filtered.%s\n", ColorRed, ColorReset)
	}
}

func cmdCopyKey(cfg *SSHConfig, reader *bufio.Reader, target string) {
	if target == "" {
		fmt.Printf("\n%sSelect server to copy key to:%s\n", ColorBold, ColorReset)
		for i, e := range cfg.Entries {
			fmt.Printf("  %s[%d]%s %s (%s@%s:%s)\n", ColorCyan, i+1, ColorReset, e.Alias, e.User, e.HostName, e.Port)
		}
		fmt.Printf("  %s[m]%s Manually enter custom IP/User\n", ColorCyan, ColorReset)
		choice := Prompt(reader, "Enter choice", "")
		if choice == "m" {
			h := Prompt(reader, "Host IP / Domain", "")
			u := Prompt(reader, "User", "root")
			p := Prompt(reader, "Port", "22")
			_, _ = EnsureDefaultSSHKey(reader)
			CopySSHKey(h, u, p, "", reader)
			return
		} else if num, err := strconv.Atoi(choice); err == nil && num >= 1 && num <= len(cfg.Entries) {
			target = cfg.Entries[num-1].Alias
		} else {
			target = choice
		}
	}

	entry := cfg.Find(target)
	if entry == nil {
		fmt.Printf("%sHost '%s' not found.%s\n", ColorRed, target, ColorReset)
		return
	}

	_, _ = EnsureDefaultSSHKey(reader)
	CopySSHKey(entry.HostName, entry.User, entry.Port, "", reader)
}

func runInteractiveMenu(cfg *SSHConfig, reader *bufio.Reader) {
	for {
		PrintBanner()
		fmt.Printf("%sWhat would you like to do?%s\n", ColorBold, ColorReset)
		fmt.Printf("  %s[1]%s Connect to a server (%shssh connect%s)\n", ColorGreen, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[2]%s Add new server & setup passwordless key (%shssh add%s)\n", ColorCyan, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[3]%s List servers & live status (%shssh list%s)\n", ColorYellow, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[4]%s Edit an existing server (%shssh edit%s)\n", ColorMagenta, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[5]%s Test passwordless authentication (%shssh test%s)\n", ColorCyan, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[6]%s Copy SSH key to server (%shssh copy-key%s)\n", ColorWhite, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[7]%s Ping server reachability (%shssh ping%s)\n", ColorGreen, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[8]%s Backup ~/.ssh/config (%shssh backup%s)\n", ColorYellow, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[9]%s Delete a server (%shssh remove%s)\n", ColorRed, ColorReset, ColorBold, ColorReset)
		fmt.Printf("  %s[0] Exit%s\n\n", ColorGray, ColorReset)

		choice := Prompt(reader, "Select an option", "1")
		switch choice {
		case "1", "connect", "c":
			cmdConnect(cfg, reader, "")
			return
		case "2", "add", "a":
			cmdAdd(cfg, reader, nil)
		case "3", "list", "ls", "l":
			cmdList(cfg, true)
			Prompt(reader, "Press Enter to continue...", "")
		case "4", "edit", "e":
			cmdEdit(cfg, reader, "")
		case "5", "test", "t":
			cmdTest(cfg, reader, "")
			Prompt(reader, "Press Enter to continue...", "")
		case "6", "copy", "copy-key":
			cmdCopyKey(cfg, reader, "")
			Prompt(reader, "Press Enter to continue...", "")
		case "7", "ping", "p":
			cmdPing(cfg, "")
			Prompt(reader, "Press Enter to continue...", "")
		case "8", "backup":
			bak, err := cfg.Backup()
			if err != nil {
				fmt.Printf("%sBackup failed: %v%s\n", ColorRed, err, ColorReset)
			} else {
				fmt.Printf("%s✓ Backup saved at %s%s\n", ColorGreen, bak, ColorReset)
			}
			Prompt(reader, "Press Enter to continue...", "")
		case "9", "remove", "rm":
			cmdRemove(cfg, reader, "")
		case "0", "q", "exit":
			fmt.Println("Goodbye!")
			return
		}
	}
}
