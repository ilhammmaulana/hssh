package main

import (
	"bufio"
	"fmt"
	"strings"
)

// ANSI Color definitions
const (
	ColorReset   = "\033[0m"
	ColorBold    = "\033[1m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorGray    = "\033[90m"
)

func PrintBanner() {
	banner := fmt.Sprintf(`%s%s
  _               _     
 | |__  ___  ___ | |__  
 | '_ \/ __|/ __|| '_ \ 
 | | | \__ \\__ \| | | |
 |_| |_|___/|___/|_| |_|
%s  Fast & Seamless OpenSSH Connection Manager
%s`, ColorCyan, ColorBold, ColorGray, ColorReset)
	fmt.Println(banner)
}

// Prompt asks the user for input, robustly handling \n, \r, and \r\n
func Prompt(reader *bufio.Reader, label string, defaultValue string) string {
	if label != "" {
		if defaultValue != "" {
			fmt.Printf("%s%s [%s%s%s]:%s ", ColorBold, label, ColorYellow, defaultValue, ColorReset+ColorBold, ColorReset)
		} else {
			fmt.Printf("%s%s:%s ", ColorBold, label, ColorReset)
		}
	}

	var sb strings.Builder
	for {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		if b == '\r' || b == '\n' {
			// Consume trailing \n if CRLF
			if b == '\r' {
				if next, err := reader.Peek(1); err == nil && len(next) > 0 && next[0] == '\n' {
					_, _ = reader.ReadByte()
				}
			}
			break
		}
		sb.WriteByte(b)
	}

	input := strings.TrimSpace(sb.String())
	if input == "" {
		return defaultValue
	}
	return input
}
