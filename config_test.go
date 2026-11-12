package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndSaveLabels(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hssh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `# Global options
ServerAliveInterval 60

# label: prod,api
Host production-api
    HostName 10.0.0.1
    User ubuntu
    Port 2222
    IdentityFile ~/.ssh/id_rsa

# tags: staging,web
Host staging-web
    HostName 10.0.0.2
    User deploy
    Port 22

# env: dev
Host local-dev
    HostName 127.0.0.1
`
	testConfigFile := filepath.Join(tempDir, "config")
	if err := os.WriteFile(testConfigFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Read file using scanner logic
	cfg := &SSHConfig{
		FilePath: testConfigFile,
		Entries:  make([]*HostEntry, 0),
		Globals:  make([]string, 0),
	}

	f, err := os.Open(testConfigFile)
	if err != nil {
		t.Fatalf("Failed to open test config: %v", err)
	}
	defer f.Close()

	// Parse using existing logic
	var currentEntry *HostEntry
	var pendingTags string

	scanner := bufioNewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		lowerTrimmed := strings.ToLower(trimmed)
		if strings.HasPrefix(lowerTrimmed, "# label:") ||
			strings.HasPrefix(lowerTrimmed, "# labels:") ||
			strings.HasPrefix(lowerTrimmed, "# tags:") ||
			strings.HasPrefix(lowerTrimmed, "# tag:") ||
			strings.HasPrefix(lowerTrimmed, "# env:") ||
			strings.HasPrefix(lowerTrimmed, "# description:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				pendingTags = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(lowerTrimmed, "host ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				alias := parts[1]
				if alias == "*" {
					cfg.Globals = append(cfg.Globals, line)
					currentEntry = nil
					continue
				}

				currentEntry = &HostEntry{
					Alias:         alias,
					Port:          "22",
					Tags:          pendingTags,
					CustomOptions: make(map[string]string),
					RawLines:      make([]string, 0),
				}
				pendingTags = ""
				cfg.Entries = append(cfg.Entries, currentEntry)
				continue
			}
		}

		if currentEntry != nil {
			currentEntry.RawLines = append(currentEntry.RawLines, line)
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				key := strings.ToLower(parts[0])
				val := strings.Join(parts[1:], " ")
				switch key {
				case "hostname":
					currentEntry.HostName = val
				case "user":
					currentEntry.User = val
				case "port":
					currentEntry.Port = val
				case "identityfile":
					currentEntry.IdentityFile = val
				default:
					currentEntry.CustomOptions[parts[0]] = val
				}
			}
		}
	}

	if len(cfg.Entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(cfg.Entries))
	}

	// Verify entries and their labels
	prod := cfg.Find("production-api")
	if prod == nil || prod.Tags != "prod,api" {
		t.Errorf("production-api tags mismatch: got %v, expected 'prod,api'", prod)
	}

	staging := cfg.Find("staging-web")
	if staging == nil || staging.Tags != "staging,web" {
		t.Errorf("staging-web tags mismatch: got %v, expected 'staging,web'", staging)
	}

	dev := cfg.Find("local-dev")
	if dev == nil || dev.Tags != "dev" {
		t.Errorf("local-dev tags mismatch: got %v, expected 'dev'", dev)
	}

	// Test saving and re-parsing
	prod.Tags = "prod,cluster,v2"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	savedBytes, err := os.ReadFile(testConfigFile)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}
	savedStr := string(savedBytes)

	if !strings.Contains(savedStr, "# label: prod,cluster,v2") {
		t.Errorf("Saved config does not contain updated label '# label: prod,cluster,v2':\n%s", savedStr)
	}
}

func bufioNewScanner(f *os.File) *bufioScannerWrapper {
	s := &bufioScannerWrapper{file: f}
	return s
}

type bufioScannerWrapper struct {
	file *os.File
	lines []string
	idx   int
}

func (b *bufioScannerWrapper) Scan() bool {
	if b.lines == nil {
		data, _ := os.ReadFile(b.file.Name())
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		b.lines = strings.Split(content, "\n")
		b.idx = 0
	}
	if b.idx < len(b.lines) {
		b.idx++
		return true
	}
	return false
}

func (b *bufioScannerWrapper) Text() string {
	if b.idx > 0 && b.idx <= len(b.lines) {
		return b.lines[b.idx-1]
	}
	return ""
}
