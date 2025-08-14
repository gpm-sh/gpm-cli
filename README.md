# GPM CLI - Game Package Manager

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/doc/install)

A modern, secure command-line interface for the GPM (Game Package Manager) registry - designed specifically for game development workflows with npm-compatible commands and Unity Package Manager (UPM) support.

## 🚀 Features

- **npm-compatible workflows** - Familiar commands for JavaScript/Node.js developers
- **Unity Package Manager (UPM) support** - Reverse-DNS package naming and Unity-specific metadata
- **Studio-scoped registries** - Multi-tenant architecture with subdomain-based access control
- **Secure authentication** - Encrypted token storage and secure credential handling
- **Professional CLI experience** - Rich terminal output, progress indicators, and helpful error messages
- **Cross-platform** - Supports Windows, macOS, and Linux

## 📦 Installation

### Quick Install (Recommended)

**One-liner installation:**

```bash
# Using curl
curl -fsSL https://gpm.sh/install.sh | bash

# Using wget
wget -qO- https://gpm.sh/install.sh | bash
```

**Custom installation:**

```bash
# Install specific version
curl -fsSL https://gpm.sh/install.sh | bash -s -- -v v0.1.0-alpha.2

# Install to custom directory
curl -fsSL https://gpm.sh/install.sh | bash -s -- -d ~/.local/bin

# Force reinstall
curl -fsSL https://gpm.sh/install.sh | bash -s -- --force
```

### Download Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/gpm-sh/gpm-cli/releases).

### Build from Source

Requirements:
- Go 1.21 or later
- Git

```bash
# Clone the repository
git clone https://github.com/gpm-sh/gpm-cli.git
cd gpm-cli

# Build and install
make build
sudo mv gpm /usr/local/bin/  # or add to your PATH

# Or install directly with Go
go install gpm.sh/gpm/gpm-cli@latest
```

### Verify Installation

```bash
gpm --version
gpm --help
```

## 🏃 Quick Start

### 1. Configure Registry

```bash
# Set your GPM registry URL
gpm config set registry https://your-studio.gpm.sh

# Or use the global registry
gpm config set registry https://gpm.sh
```

### 2. Authenticate

```bash
# Register a new account
gpm register

# Login with existing credentials
gpm login
```

### 3. Install Packages

```bash
# Install a package
gpm install com.unity.ugui

# Install specific version
gpm install com.unity.ugui@1.0.0

# Install and save to package.json
gpm install --save com.company.analytics
gpm install --save-dev com.company.test-utils
```

### 4. Publish Packages

```bash
# Create a package tarball
gpm pack

# Publish to registry
gpm publish your-package-1.0.0.tgz
```

## 📚 Commands Reference

### Package Management

| Command | Description | Example |
|---------|-------------|---------|
| `gpm install [package]` | Install packages | `gpm install com.unity.ugui@1.0.0` |
| `gpm uninstall <package>` | Remove packages | `gpm uninstall com.unity.ugui` |
| `gpm list` | List installed packages | `gpm list --production` |
| `gpm info <package>` | Show package information | `gpm info com.unity.ugui` |
| `gpm search <term>` | Search for packages | `gpm search analytics` |

### Publishing

| Command | Description | Example |
|---------|-------------|---------|
| `gpm pack` | Create package tarball | `gpm pack` |
| `gpm publish <tarball>` | Publish package | `gpm publish my-package-1.0.0.tgz` |

### Authentication

| Command | Description | Example |
|---------|-------------|---------|
| `gpm register` | Create new account | `gpm register` |
| `gpm login` | Authenticate with registry | `gpm login` |
| `gpm logout` | Clear authentication | `gpm logout` |
| `gpm whoami` | Show current user | `gpm whoami` |

### Configuration

| Command | Description | Example |
|---------|-------------|---------|
| `gpm config set <key> <value>` | Set configuration | `gpm config set registry https://gpm.sh` |
| `gpm config get <key>` | Get configuration | `gpm config get registry` |
| `gpm config list` | List all settings | `gpm config list` |

### Utilities

| Command | Description | Example |
|---------|-------------|---------|
| `gpm version` | Show CLI version | `gpm version` |
| `gpm help [command]` | Show help | `gpm help install` |

## 🔧 Global Flags

All commands support these global flags:

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Enable verbose output |
| `--debug` | Enable debug output |
| `--quiet, -q` | Suppress non-essential output |
| `--json` | Output in JSON format |

## 📋 Package.json Structure

GPM supports standard npm `package.json` with Unity-specific extensions:

```json
{
  "name": "com.company.my-package",
  "version": "1.0.0",
  "displayName": "My Unity Package",
  "description": "A Unity package for amazing features",
  "unity": "2022.3",
  "license": "MIT",
  "author": {
    "name": "Your Name",
    "email": "your.email@company.com"
  },
  "dependencies": {
    "com.unity.ugui": "1.0.0"
  },
  "devDependencies": {
    "com.company.test-utils": "^2.0.0"
  },
  "keywords": ["unity", "ui", "game-development"]
}
```

### Required Fields

- `name` - Reverse-DNS package name (e.g., `com.company.package`)
- `version` - Semantic version (e.g., `1.0.0`)
- `description` - Package description

### Unity-Specific Fields

- `displayName` - Human-readable name shown in Unity
- `unity` - Minimum Unity version (e.g., `2022.3`)

## 🏗️ Development

### Building

```bash
# Install dependencies
make deps

# Format code
make fmt

# Run linter
make lint

# Build binary
make build

# Build for all platforms
make build-all
```

### Testing

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests
make test-integration

# Generate coverage report
make test-coverage
```

### Version Management

Version information is automatically injected at build time:

```bash
# Show build variables
make version

# Build with custom version
VERSION=1.2.3 make build
```

## 🔐 Security

### Secure Credential Handling

- Passwords are cleared from memory immediately after use
- Input validation prevents injection attacks  
- HTTPS-only communication with registries
- Secure file permissions on configuration files

## 🌍 Configuration

### Configuration File

GPM stores configuration in `~/.gpmrc`:

```yaml
registry: https://gpm.sh
username: your-username
token: your-auth-token
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GPM_REGISTRY` | Registry URL | `https://gpm.sh` |
| `GPM_TOKEN` | Authentication token | - |
| `NO_COLOR` | Disable colored output | - |

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Run the test suite (`make test`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

### Development Guidelines

- Follow Go best practices and idioms
- Add tests for all new functionality
- Update documentation for user-facing changes
- Use conventional commit messages
- Ensure all tests pass before submitting

## 📄 License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.

## 🆘 Support

- **Documentation**: [https://docs.gpm.sh](https://docs.gpm.sh)
- **Issues**: [GitHub Issues](https://github.com/gpm-sh/gpm-cli/issues)
- **Discussions**: [GitHub Discussions](https://github.com/gpm-sh/gpm-cli/discussions)

## 🗺️ Roadmap

- [ ] Package dependency resolution and lock files
- [ ] Workspace support for monorepos
- [ ] Plugin system for custom commands
- [ ] Integration with CI/CD platforms
- [ ] Advanced package validation rules
- [ ] Offline package caching
