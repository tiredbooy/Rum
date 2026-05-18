#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════
#  Colors & Styles
# ═══════════════════════════════════════════════════════════
BOLD='\033[1m'
RESET='\033[0m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
WHITE='\033[1;37m'
BG_BLUE='\033[44m'
BG_GREEN='\033[42m'
BG_RED='\033[41m'
BG_CYAN='\033[46m'
BG_MAGENTA='\033[35m'

print_box() {
    local color="$1" text="$2"
    local width=60
    local len=${#text}
    local pad=$(( (width - len - 2) / 2 ))
    local extra=$(( (width - len - 2) % 2 ))
    echo -e "${color}╔$(printf '═%.0s' $(seq 1 $width))╗${RESET}"
    echo -e "${color}║$(printf ' %.0s' $(seq 1 $pad))${text}$(printf ' %.0s' $(seq 1 $((pad + extra))))║${RESET}"
    echo -e "${color}╚$(printf '═%.0s' $(seq 1 $width))╝${RESET}"
}

spinner() {
    local pid=$1
    local spinstr='◐◓◑◒'
    while kill -0 "$pid" 2>/dev/null; do
        for (( i=0; i<${#spinstr}; i++ )); do
            printf "\r  %c" "${spinstr:$i:1}"
            sleep 0.1
        done
    done
    printf "\r  \r"
}

run_with_spinner() {
    local msg="$1"
    shift
    ("$@") &
    local pid=$!
    echo -ne "${msg} "
    spinner $pid
    wait $pid
    echo -e "${GREEN}✓${RESET}"
}

# ═══════════════════════════════════════════════════════════
#  Find the project root (directory containing go.mod)
# ═══════════════════════════════════════════════════════════
PROJECT_ROOT="$(pwd)"
while [[ ! -f "$PROJECT_ROOT/go.mod" ]]; do
    if [[ "$PROJECT_ROOT" == "/" ]]; then
        echo -e "${BG_RED} ERROR ${RESET} Cannot find the Rum project root (no go.mod)."
        echo "Make sure you run this script from inside the cloned repository."
        exit 1
    fi
    PROJECT_ROOT="$(dirname "$PROJECT_ROOT")"
done
cd "$PROJECT_ROOT"

# ═══════════════════════════════════════════════════════════
#  Welcome
# ═══════════════════════════════════════════════════════════
clear
echo ""
echo -e "${MAGENTA}${BOLD}          R U M   -   I N S T A L L E R${RESET}"
echo ""
print_box "$BG_MAGENTA" " Smart CLI Download Manager "
echo ""
echo -e "${CYAN}This script will build and install Rum on your system.${RESET}"
echo ""

# ═══════════════════════════════════════════════════════════
#  Check prerequisites
# ═══════════════════════════════════════════════════════════
echo -e "${BOLD}▶ Checking prerequisites …${RESET}"
command -v go >/dev/null 2>&1 || {
    echo -e "${BG_RED} ERROR ${RESET} Go is not installed."
    echo "Please install Go from https://go.dev/doc/install and then re-run this script."
    exit 1
}
echo -e "  ${GREEN}✓ Go $(go version | awk '{print $3}')${RESET}"

# Verify we're in a buildable directory
if [[ ! -d "cmd/rum" ]]; then
    echo -e "${BG_RED} ERROR ${RESET} Could not find cmd/rum in the project root."
    echo "Ensure you have the correct repository structure."
    exit 1
fi
echo -e "  ${GREEN}✓ Repository structure OK${RESET}"
echo ""

# ═══════════════════════════════════════════════════════════
#  Build confirmation
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}Ready to build Rum?${RESET} (${BOLD}Y${RESET}es / ${BOLD}n${RESET}o) [Y/n]"
read -r build_choice
build_choice=${build_choice:-y}
if [[ ! "$build_choice" =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Build cancelled. Exiting.${RESET}"
    exit 0
fi

# ═══════════════════════════════════════════════════════════
#  Optional Iranian mirror for Go modules
# ═══════════════════════════════════════════════════════════
echo ""
echo -e "${YELLOW}Would you like to use a Iranian mirror for downloading Go modules?${RESET}"
echo "  (This can greatly speed up downloads And bypass Internet restriction for users in Iran) [Y/n]"
read -r mirror_choice
mirror_choice=${mirror_choice:-y}

BUILD_CMD="go build -o rum ./cmd/rum"
if [[ "$mirror_choice" =~ ^[Yy]$ ]]; then
    MIRROR_URL="https://mirror-go.runflare.com"
    echo -e "  ${CYAN}✓ Using mirror: ${MIRROR_URL}${RESET}"
    BUILD_CMD="env GOPROXY=${MIRROR_URL} ${BUILD_CMD}"
else
    echo -e "  ${CYAN}Using default Go proxy (or direct connection)${RESET}"
fi
echo ""

# ═══════════════════════════════════════════════════════════
#  Build
# ═══════════════════════════════════════════════════════════
run_with_spinner "Building Rum binary …" ${BUILD_CMD}

if [[ ! -f "rum" ]]; then
    echo -e "${BG_RED} ERROR ${RESET} Build produced no executable."
    exit 1
fi
size=$(du -h rum | cut -f1)
echo -e "  ${GREEN}✓ Binary created (${size})${RESET}"
echo ""

# ═══════════════════════════════════════════════════════════
#  Install directory
# ═══════════════════════════════════════════════════════════
INSTALL_DIR="$HOME/bin"
echo -e "${BOLD}▶ Preparing installation directory …${RESET}"
mkdir -p "$INSTALL_DIR"
echo -e "  ${GREEN}✓ ${INSTALL_DIR}${RESET}"

if [[ -f "$INSTALL_DIR/rum" ]]; then
    echo -e "${YELLOW}An existing Rum binary was found. Overwrite?${RESET} [Y/n]"
    read -r overwrite
    overwrite=${overwrite:-y}
    if [[ ! "$overwrite" =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Installation aborted.${RESET}"
        exit 0
    fi
fi

cp -f rum "$INSTALL_DIR/rum"
chmod +x "$INSTALL_DIR/rum"
echo -e "  ${GREEN}✓ Binary installed to ${INSTALL_DIR}/rum${RESET}"
echo ""

# ═══════════════════════════════════════════════════════════
#  PATH configuration
# ═══════════════════════════════════════════════════════════
add_to_path() {
    local rc_file="$1"
    local shell_name="$2"
    if grep -q 'export PATH="$HOME/bin:$PATH"' "$rc_file" 2>/dev/null; then
        echo -e "  ${CYAN}Already present in ${rc_file}${RESET}"
        return 0
    fi
    echo "" >> "$rc_file"
    echo "# Added by Rum installer" >> "$rc_file"
    echo 'export PATH="$HOME/bin:$PATH"' >> "$rc_file"
    echo -e "  ${GREEN}✓ Added to ${rc_file}${RESET}"
}

echo -e "${YELLOW}Would you like to add Rum to your PATH?${RESET} (${BOLD}Y${RESET}es / ${BOLD}n${RESET}o) [Y/n]"
read -r path_choice
path_choice=${path_choice:-y}

if [[ "$path_choice" =~ ^[Yy]$ ]]; then
    echo -e "${BOLD}▶ Configuring PATH …${RESET}"
    case "$SHELL" in
        */bash) add_to_path "$HOME/.bashrc" "bash" ;;
        */zsh)  add_to_path "$HOME/.zshrc" "zsh" ;;
        *)
            echo -e "${YELLOW}Unknown shell. Please add ${INSTALL_DIR} to your PATH manually.${RESET}"
            ;;
    esac
else
    echo -e "${YELLOW}Skipping PATH setup. Add ${INSTALL_DIR} to your PATH manually if needed.${RESET}"
fi
echo ""

# ═══════════════════════════════════════════════════════════
#  Cleanup & Finish
# ═══════════════════════════════════════════════════════════
rm -f rum

clear
echo ""
print_box "$BG_GREEN" " Installation Complete! "
echo ""
echo -e "${WHITE}${BOLD}Rum is now installed on your system!${RESET}"
echo ""
echo -e "To start using it immediately, either:"
echo -e "  1. Open a new terminal window"
echo -e "  2. Run: ${CYAN}source ~/.bashrc${RESET} (or ~/.zshrc)"
echo ""
echo -e "Then try: ${CYAN}rum --help${RESET}"
echo ""
echo -e "${YELLOW}Happy downloading! 🚀${RESET}"
echo ""