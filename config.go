package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HostEntry struct {
	Alias         string
	HostName      string
	User          string
	Port          string
	IdentityFile  string
	Tags          string
	CustomOptions map[string]string
	RawLines      []string
}

type SSHConfig struct {
	FilePath string
	Entries  []*HostEntry
	Globals  []string
}

func GetSSHDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", err
	}
	return sshDir, nil
}

func GetConfigPath() (string, error) {
	sshDir, err := GetSSHDir()
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(sshDir, "config")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		file, err := os.OpenFile(cfgPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return "", err
		}
		file.Close()
	} else {
		_ = os.Chmod(cfgPath, 0600)
	}
	return cfgPath, nil
}

func LoadSSHConfig() (*SSHConfig, error) {
	cfgPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadSSHConfigFile(cfgPath)
}

func LoadSSHConfigFile(cfgPath string) (*SSHConfig, error) {
	cfg := &SSHConfig{
		FilePath: cfgPath,
		Entries:  make([]*HostEntry, 0),
		Globals:  make([]string, 0),
	}

	file, err := os.Open(cfgPath)
	if err != nil {
		return cfg, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentEntry *HostEntry
	var pendingTags string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for label comments
		if strings.HasPrefix(trimmed, "#") {
			commentContent := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			lowerComment := strings.ToLower(commentContent)

			var extractedVal string
			matched := false

			prefixes := []string{
				"label:", "labels:", "tag:", "tags:", "env:", "description:", "desc:",
				"label ", "labels ", "tag ", "tags ", "env ",
			}
			for _, p := range prefixes {
				if strings.HasPrefix(lowerComment, p) {
					if strings.Contains(commentContent, ":") {
						parts := strings.SplitN(commentContent, ":", 2)
						if len(parts) == 2 {
							extractedVal = strings.TrimSpace(parts[1])
						}
					} else {
						spaceParts := strings.Fields(commentContent)
						if len(spaceParts) >= 2 {
							extractedVal = strings.Join(spaceParts[1:], " ")
						}
					}
					matched = true
					break
				}
			}

			if matched {
				isIndented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
				if isIndented && currentEntry != nil {
					currentEntry.Tags = extractedVal
				} else {
					if currentEntry != nil {
						cfg.Entries = append(cfg.Entries, currentEntry)
						currentEntry = nil
					}
					pendingTags = extractedVal
				}
				continue
			}

			if currentEntry != nil {
				currentEntry.RawLines = append(currentEntry.RawLines, line)
			} else {
				cfg.Globals = append(cfg.Globals, line)
			}
			continue
		}

		if trimmed == "" {
			if currentEntry != nil {
				currentEntry.RawLines = append(currentEntry.RawLines, line)
			} else {
				cfg.Globals = append(cfg.Globals, line)
			}
			continue
		}

		parts := strings.Fields(trimmed)
		if len(parts) == 0 {
			continue
		}

		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = strings.Join(parts[1:], " ")
		}

		if strings.EqualFold(key, "Host") {
			if currentEntry != nil {
				cfg.Entries = append(cfg.Entries, currentEntry)
			}
			currentEntry = &HostEntry{
				Alias:         val,
				Port:          "22",
				Tags:          pendingTags,
				CustomOptions: make(map[string]string),
				RawLines:      []string{line},
			}
			pendingTags = ""
		} else {
			if currentEntry != nil {
				currentEntry.RawLines = append(currentEntry.RawLines, line)
				switch strings.ToLower(key) {
				case "hostname":
					currentEntry.HostName = val
				case "user":
					currentEntry.User = val
				case "port":
					currentEntry.Port = val
				case "identityfile":
					currentEntry.IdentityFile = val
				case "tag", "tags", "label", "labels", "env":
					currentEntry.Tags = val
				default:
					currentEntry.CustomOptions[key] = val
				}
			} else {
				cfg.Globals = append(cfg.Globals, line)
			}
		}
	}

	if currentEntry != nil {
		cfg.Entries = append(cfg.Entries, currentEntry)
	}

	return cfg, nil
}

func (cfg *SSHConfig) Backup() (string, error) {
	if _, err := os.Stat(cfg.FilePath); os.IsNotExist(err) {
		return "", nil
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.bak.%s", cfg.FilePath, timestamp)

	src, err := os.Open(cfg.FilePath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return backupPath, nil
}

func (cfg *SSHConfig) Find(alias string) *HostEntry {
	for _, e := range cfg.Entries {
		if strings.EqualFold(e.Alias, alias) {
			return e
		}
	}
	return nil
}

func (cfg *SSHConfig) Remove(alias string) bool {
	for i, e := range cfg.Entries {
		if strings.EqualFold(e.Alias, alias) {
			cfg.Entries = append(cfg.Entries[:i], cfg.Entries[i+1:]...)
			return true
		}
	}
	return false
}

func (cfg *SSHConfig) AddOrUpdate(entry *HostEntry) {
	existing := cfg.Find(entry.Alias)
	if existing != nil {
		existing.HostName = entry.HostName
		existing.User = entry.User
		existing.Port = entry.Port
		existing.IdentityFile = entry.IdentityFile
		existing.Tags = entry.Tags
		for k, v := range entry.CustomOptions {
			existing.CustomOptions[k] = v
		}
	} else {
		cfg.Entries = append(cfg.Entries, entry)
	}
}

func (cfg *SSHConfig) Save() error {
	// Automatically create a safety backup before writing
	_, _ = cfg.Backup()

	file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// Write globals if any
	for _, g := range cfg.Globals {
		_, _ = writer.WriteString(g + "\n")
	}

	// Write host blocks
	for i, e := range cfg.Entries {
		if e.Tags != "" {
			_, _ = writer.WriteString(fmt.Sprintf("# label: %s\n", e.Tags))
		}
		_, _ = writer.WriteString(fmt.Sprintf("Host %s\n", e.Alias))
		if e.HostName != "" {
			_, _ = writer.WriteString(fmt.Sprintf("    HostName %s\n", e.HostName))
		}
		if e.User != "" {
			_, _ = writer.WriteString(fmt.Sprintf("    User %s\n", e.User))
		}
		if e.Port != "" && e.Port != "22" {
			_, _ = writer.WriteString(fmt.Sprintf("    Port %s\n", e.Port))
		}
		if e.IdentityFile != "" {
			_, _ = writer.WriteString(fmt.Sprintf("    IdentityFile %s\n", e.IdentityFile))
		}
		for k, v := range e.CustomOptions {
			_, _ = writer.WriteString(fmt.Sprintf("    %s %s\n", k, v))
		}

		if i < len(cfg.Entries)-1 {
			_, _ = writer.WriteString("\n")
		}
	}

	return writer.Flush()
}
