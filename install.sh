#!/bin/bash

# GPM CLI Installation Script
# Usage: curl -fsSL https://gpm.sh/install.sh | bash
# Or: wget -qO- https://gpm.sh/install.sh | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
REPO="gpm-sh/gpm-cli"
BINARY_NAME="gpm"
INSTALL_DIR="/usr/local/bin"

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${PURPLE}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "                           GPM CLI Installer                                 "
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${NC}"
}

print_footer() {
    echo -e "${PURPLE}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${NC}"
}

# Detect platform
detect_platform() {
    local os=""
    local arch=""

    # Detect OS
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)          
            print_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        arm64|aarch64)  arch="arm64" ;;
        i386|i686)      arch="386" ;;
        armv7l)         arch="arm" ;;
        *)              
            print_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac

    # Set binary name for Windows
    if [ "$os" = "windows" ]; then
        BINARY_NAME="gpm.exe"
    fi

    echo "${os}_${arch}"
}

# Get latest release version (including pre-releases)
get_latest_version() {
    # Output status to stderr so it doesn't interfere with version capture
    print_status "Fetching latest release information..." >&2
    
    local api_url="https://api.github.com/repos/${REPO}/releases"
    
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$api_url" | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$api_url" | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/'
    else
        print_error "Neither curl nor wget is available. Please install one of them." >&2
        exit 1
    fi
}

# Download and install binary
install_binary() {
    local version=$1
    local platform=$2
    local temp_dir=$(mktemp -d)
    # Convert platform format: darwin_arm64 -> darwin-arm64
    local platform_fixed=$(echo "$platform" | sed 's/_/-/g')
    local binary_name="gpm-${platform_fixed}"
    
    # Add .exe extension for Windows
    if [[ "$platform" == *"windows"* ]]; then
        binary_name="${binary_name}.exe"
    fi
    local download_url="https://github.com/${REPO}/releases/download/${version}/${binary_name}"

    print_status "Downloading GPM CLI ${version} for ${platform}..."
    
    # Download the binary directly
    if command -v curl >/dev/null 2>&1; then
        if ! curl -fsSL "$download_url" -o "${temp_dir}/${BINARY_NAME}"; then
            print_error "Failed to download GPM CLI"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -q "$download_url" -O "${temp_dir}/${BINARY_NAME}"; then
            print_error "Failed to download GPM CLI"
            exit 1
        fi
    fi

    print_status "Binary downloaded successfully..."
    
    # Make binary executable
    chmod +x "${temp_dir}/${BINARY_NAME}"

    # Determine install directory
    local install_path="$INSTALL_DIR"
    if [ ! -w "$INSTALL_DIR" ] && [ "$EUID" -ne 0 ]; then
        print_warning "Cannot write to $INSTALL_DIR without sudo privileges"
        
        # Try user's local bin directory
        local user_bin="$HOME/.local/bin"
        if [ ! -d "$user_bin" ]; then
            print_status "Creating directory: $user_bin"
            mkdir -p "$user_bin"
        fi
        install_path="$user_bin"
        
        print_status "Installing to user directory: $install_path"
        
        # Add to PATH if not already there
        local shell_profile=""
        if [ -n "$BASH_VERSION" ]; then
            shell_profile="$HOME/.bashrc"
        elif [ -n "$ZSH_VERSION" ]; then
            shell_profile="$HOME/.zshrc"
        else
            shell_profile="$HOME/.profile"
        fi
        
        if [ -f "$shell_profile" ] && ! grep -q "$user_bin" "$shell_profile"; then
            echo "export PATH=\"$user_bin:\$PATH\"" >> "$shell_profile"
            print_status "Added $user_bin to PATH in $shell_profile"
            print_warning "Please restart your shell or run: source $shell_profile"
        fi
    else
        print_status "Installing to system directory: $install_path"
    fi

    # Install the binary
    local binary_source="${temp_dir}/${BINARY_NAME}"
    local binary_dest="${install_path}/${BINARY_NAME}"

    if [ -f "$binary_source" ]; then
        if [ -w "$install_path" ] || [ "$EUID" -eq 0 ]; then
            cp "$binary_source" "$binary_dest"
            chmod +x "$binary_dest"
        else
            print_status "Requesting sudo privileges to install to $install_path..."
            sudo cp "$binary_source" "$binary_dest"
            sudo chmod +x "$binary_dest"
        fi
        
        print_success "GPM CLI installed successfully to: $binary_dest"
    else
        print_error "Binary not found in archive: $binary_source"
        exit 1
    fi

    # Cleanup
    rm -rf "$temp_dir"
}

# Verify installation
verify_installation() {
    print_status "Verifying installation..."
    
    if command -v gpm >/dev/null 2>&1; then
        local version=$(gpm version --json 2>/dev/null | grep -o '"version":"[^"]*"' | cut -d'"' -f4 2>/dev/null || gpm version 2>/dev/null | head -1 || echo "unknown")
        print_success "GPM CLI is successfully installed!"
        echo -e "${CYAN}Version: ${version}${NC}"
        echo -e "${CYAN}Location: $(which gpm)${NC}"
        
        echo ""
        print_status "Quick start guide:"
        echo -e "  ${GREEN}gpm register${NC}     - Create a new account"
        echo -e "  ${GREEN}gpm login${NC}        - Login to your account"
        echo -e "  ${GREEN}gpm init${NC}         - Initialize a new package"
        echo -e "  ${GREEN}gpm pack${NC}         - Create a package tarball"
        echo -e "  ${GREEN}gpm publish${NC}      - Publish package to registry"
        echo -e "  ${GREEN}gpm install <pkg>${NC} - Install a package"
        echo -e "  ${GREEN}gpm --help${NC}       - Show all available commands"
        
        echo ""
        print_status "Documentation: https://github.com/gpm-sh/gpm-cli"
        print_status "Registry: https://gpm.sh"
        
    else
        print_error "Installation verification failed. GPM CLI is not in your PATH."
        print_warning "You may need to restart your shell or add the installation directory to your PATH manually."
        exit 1
    fi
}

# Handle command line arguments
show_help() {
    echo "GPM CLI Installation Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -v, --version VERSION    Install specific version (default: latest)"
    echo "  -d, --dir DIRECTORY      Install directory (default: /usr/local/bin)"
    echo "  --force                  Force installation even if already installed"
    echo "  -h, --help               Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                       Install latest version"
    echo "  $0 -v v0.1.0-alpha.2     Install specific version"
    echo "  $0 -d ~/.local/bin       Install to custom directory"
    echo ""
    echo "One-liner installation:"
    echo "  curl -fsSL https://gpm.sh/install.sh | bash"
    echo "  wget -qO- https://gpm.sh/install.sh | bash"
}

# Parse command line arguments
VERSION=""
FORCE_INSTALL=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -d|--dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --force)
            FORCE_INSTALL=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Main installation process
main() {
    print_header
    
    # Check if already installed
    if command -v gpm >/dev/null 2>&1 && [ "$FORCE_INSTALL" = false ]; then
        local current_version=$(gpm version 2>/dev/null | head -1 || echo "unknown")
        print_warning "GPM CLI is already installed: $current_version"
        print_status "Use --force to reinstall or -v VERSION to install a different version"
        exit 0
    fi

    # Detect platform
    local platform=$(detect_platform)
    print_status "Detected platform: $platform"

    # Get version to install
    if [ -z "$VERSION" ]; then
        VERSION=$(get_latest_version)
        if [ -z "$VERSION" ]; then
            print_error "Failed to fetch latest version"
            exit 1
        fi
    fi
    print_status "Installing version: $VERSION"

    # Install binary
    install_binary "$VERSION" "$platform"
    
    # Verify installation
    verify_installation
    
    print_footer
    print_success "🎉 GPM CLI installation completed successfully!"
}

# Run main function
main "$@"
