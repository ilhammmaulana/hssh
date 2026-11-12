package main

import (
	"bufio"
	"fmt"
	"os"
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

// Prompt asks the user for input with an optional default value
func Prompt(reader *bufio.Reader, label string, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s%s [%s%s%s]:%s ", ColorBold, label, ColorYellow, defaultValue, ColorReset+ColorBold, ColorReset)
	} else {
		fmt.Printf("%s%s:%s ", ColorBold, label, ColorReset)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("\nAborted.")
		os.Exit(0)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}
