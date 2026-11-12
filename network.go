package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func GetDefaultKeyPaths() (string, string, error) {
	sshDir, err := GetSSHDir()
	if err != nil {
		return "", "", err
	}
	privKey := filepath.Join(sshDir, "id_ed25519")
	pubKey := filepath.Join(sshDir, "id_ed25519.pub")
	return privKey, pubKey, nil
}

func GetCurrentUsername() string {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("USERNAME")
	}
	if u == "" {
		u = "user"
	}
	return u
}

// EnsureDefaultSSHKey checks if ~/.ssh/id_ed25519 exists, offers to generate if missing
func EnsureDefaultSSHKey(reader *bufio.Reader) (string, error) {
	privKey, _, err := GetDefaultKeyPaths()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(privKey); os.IsNotExist(err) {
		fmt.Printf("\n%s[!] No default SSH key (ed25519) found at %s%s\n", ColorYellow, privKey, ColorReset)
		opt := strings.ToLower(Prompt(reader, "Generate modern ed25519 key pair now? (Y/n)", "Y"))
		if opt == "y" || opt == "yes" || opt == "" {
			user := GetCurrentUsername()
			comment := Prompt(reader, "Key label / comment", user+"@hssh")
			fmt.Printf("%sGenerating ed25519 key pair...%s\n", ColorCyan, ColorReset)

			cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", comment, "-f", privKey, "-N", "")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("failed to generate key: %w", err)
			}
			fmt.Printf("%s✓ SSH Key pair successfully created at %s%s\n\n", ColorGreen, privKey, ColorReset)
		}
	}

	return privKey, nil
}

// PingHost performs a fast TCP dial to check reachability and latency
func PingHost(host string, port string, timeout time.Duration) (bool, time.Duration) {
	if port == "" {
		port = "22"
	}
	address := net.JoinHostPort(host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false, 0
	}
	defer conn.Close()
	return true, time.Since(start)
}

// CopySSHKey copies public key via ssh-copy-id (if available) or direct SSH command (cross-platform / Windows)
func CopySSHKey(host, user, port, pubKeyPath string, reader *bufio.Reader) bool {
	if port == "" {
		port = "22"
	}
	if pubKeyPath == "" {
		_, defaultPub, _ := GetDefaultKeyPaths()
		pubKeyPath = defaultPub
	}

	// Expand ~ if present
	if strings.HasPrefix(pubKeyPath, "~") {
		home, _ := os.UserHomeDir()
		pubKeyPath = filepath.Join(home, pubKeyPath[1:])
	}

	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		fmt.Printf("%s[!] Public key file not found: %s%s\n", ColorRed, pubKeyPath, ColorReset)
		return false
	}

	target := fmt.Sprintf("%s@%s", user, host)
	fmt.Printf("\n%sInstalling public key to %s (port %s)...%s\n", ColorCyan, target, port, ColorReset)
	fmt.Printf("%sNote: If prompted, enter the remote user's password ONE LAST TIME.%s\n\n", ColorYellow, ColorReset)

	// Check if ssh-copy-id exists (common on Linux/macOS)
	if _, err := exec.LookPath("ssh-copy-id"); err == nil && runtime.GOOS != "windows" {
		cmd := exec.Command("ssh-copy-id", "-i", pubKeyPath, "-p", port, target)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err == nil {
			fmt.Printf("\n%s✓ Public key successfully installed! Passwordless login is ACTIVE.%s\n\n", ColorGreen, ColorReset)
			return true
		}
		fmt.Printf("\n%sssh-copy-id failed. Attempting universal fallback method...%s\n", ColorYellow, ColorReset)
	}

	// Universal fallback method (works on Windows, Linux, macOS)
	pubBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		fmt.Printf("%sFailed to read public key: %v%s\n", ColorRed, err, ColorReset)
		return false
	}
	pubKeyContent := strings.TrimSpace(string(pubBytes))
	remoteCmd := fmt.Sprintf("mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys", pubKeyContent)
	
	fbCmd := exec.Command("ssh", "-p", port, target, remoteCmd)
	fbCmd.Stdin = os.Stdin
	fbCmd.Stdout = os.Stdout
	fbCmd.Stderr = os.Stderr
	if err := fbCmd.Run(); err == nil {
		fmt.Printf("\n%s✓ Key installation succeeded! Passwordless login is ACTIVE.%s\n\n", ColorGreen, ColorReset)
		return true
	}

	fmt.Printf("%s[!] Failed to install public key automatically.%s\n", ColorRed, ColorReset)
	return false
}

// TestPasswordless checks if passwordless SSH works
func TestPasswordless(alias string) (bool, string) {
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=4", alias, "echo HSSH_OK")
	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	if err == nil && strings.Contains(outputStr, "HSSH_OK") {
		return true, ""
	}
	return false, strings.TrimSpace(outputStr)
}

// ConnectSSH hands over stdin/stdout/stderr directly to native ssh
func ConnectSSH(alias string) {
	fmt.Printf("%sConnecting to %s%s%s...%s\n", ColorGreen, ColorBold, alias, ColorReset+ColorGreen, ColorReset)
	cmd := exec.Command("ssh", alias)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
