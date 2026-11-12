#!/usr/bin/env bash
# ==============================================================================
# hssh Universal Installer (Linux & macOS)
# One-liner: curl -fsSL https://raw.githubusercontent.com/ilhammmaulana/hssh/main/install.sh | bash
# ==============================================================================

set -e

# Terminal Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

print_banner() {
    echo -e "${CYAN}${BOLD}"
    cat << "EOF"
  _               _     
 | |__  ___  ___ | |__  
 | '_ \/ __|/ __|| '_ \ 
 | | | \__ \\__ \| | | |
 |_| |_|___/|___/|_| |_|
EOF
    echo -e "${NC}${YELLOW}  Fast & Seamless OpenSSH Connection Manager${NC}"
    echo -e "${BLUE}  ==========================================${NC}\n"
}

detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64|amd64)
            GOARCH="amd64"
            ;;
        aarch64|arm64)
            GOARCH="arm64"
            ;;
        *)
            echo -e "${RED}[ERROR] Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac

    case "$OS" in
        linux)
            GOOS="linux"
            ;;
        darwin)
            GOOS="darwin"
            ;;
        *)
            echo -e "${RED}[ERROR] Unsupported OS: $OS. For Windows, please run install.ps1.${NC}"
            exit 1
            ;;
    esac

    BINARY_NAME="hssh-${GOOS}-${GOARCH}"
}

check_dependencies() {
    echo -e "${BOLD}Checking requirements...${NC}"
    
    if ! command -v ssh >/dev/null 2>&1; then
        echo -e "${RED}[ERROR] 'ssh' binary not found. Please install OpenSSH client.${NC}"
        exit 1
    fi

    echo -e "${GREEN}✓ Core OpenSSH client found.${NC}"
}

determine_install_dir() {
    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    elif [ -d "$HOME/.local/bin" ]; then
        INSTALL_DIR="$HOME/.local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
}

install_binary() {
    determine_install_dir
    TARGET_BIN="$INSTALL_DIR/hssh"

    echo -e "\n${BOLD}Installing hssh (${CYAN}${GOOS}/${GOARCH}${NC}) into ${CYAN}${INSTALL_DIR}${NC}..."

    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd)"
    LOCAL_PREBUILT="$SCRIPT_DIR/dist/$BINARY_NAME"
    LOCAL_BIN="$SCRIPT_DIR/hssh"

    if [ -f "$LOCAL_PREBUILT" ]; then
        rm -f "$TARGET_BIN"
        cp "$LOCAL_PREBUILT" "$TARGET_BIN"
    elif [ -f "$LOCAL_BIN" ] && [ "$TARGET_BIN" != "$LOCAL_BIN" ]; then
        rm -f "$TARGET_BIN"
        cp "$LOCAL_BIN" "$TARGET_BIN"
    else
        REPO_URL="https://github.com/ilhammmaulana/hssh/releases/latest/download/${BINARY_NAME}"
        echo -e "Downloading latest release (${BINARY_NAME}) from GitHub..."
        curl -fsSL "$REPO_URL" -o "$TARGET_BIN"
    fi

    chmod +x "$TARGET_BIN"
    echo -e "${GREEN}✓ Binary installed at: ${TARGET_BIN}${NC}"

    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *)
            echo -e "\n${YELLOW}[!] Warning: ${INSTALL_DIR} is not currently in your \$PATH.${NC}"
            echo -e "Add this to your ~/.bashrc or ~/.zshrc:"
            echo -e "    ${BOLD}export PATH=\"\$PATH:${INSTALL_DIR}\"${NC}"
            ;;
    esac
}

setup_ssh_environment() {
    echo -e "\n${BOLD}Verifying SSH environment...${NC}"
    mkdir -p "$HOME/.ssh"
    chmod 700 "$HOME/.ssh"
    
    if [ ! -f "$HOME/.ssh/config" ]; then
        touch "$HOME/.ssh/config"
        chmod 600 "$HOME/.ssh/config"
        echo -e "${GREEN}✓ Created ~/.ssh/config with 0600 permissions.${NC}"
    fi
}

print_success() {
    echo -e "\n${GREEN}${BOLD}====================================================${NC}"
    echo -e "${GREEN}${BOLD}✓ hssh has been successfully installed!${NC}"
    echo -e "${GREEN}${BOLD}====================================================${NC}\n"
    echo -e "Get started with:"
    echo -e "  ${CYAN}${BOLD}hssh${NC}          # Launch interactive manager"
    echo -e "  ${CYAN}${BOLD}hssh add${NC}      # Add your first remote server"
    echo -e "  ${CYAN}${BOLD}hssh list${NC}     # View your configured servers"
    echo -e "  ${CYAN}${BOLD}hssh help${NC}     # View full documentation\n"
}

main() {
    print_banner
    detect_platform
    check_dependencies
    install_binary
    setup_ssh_environment
    print_success
}

main "$@"
