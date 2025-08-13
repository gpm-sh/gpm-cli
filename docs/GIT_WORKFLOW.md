# Git Workflow Guide for GPM CLI

This document outlines the standardized Git workflow, branching strategy, commit conventions, and release process for the GPM CLI project.

## 🌳 Branching Strategy

We follow **Git Flow** with some modern adaptations:

```
main     ──●──────●──────●──────●──     (Production releases)
            │      │      │      │
develop  ──●──●──●──●──●──●──●──●──     (Integration branch)
           │  │     │     │  │
feature/   ──●──●───┘     │  │          (New features)
hotfix/           ───●────┘  │          (Critical fixes)
release/              ───●───┘          (Release preparation)
```

### Branch Types

| Branch Type | Purpose | Base Branch | Merge To | Naming |
|-------------|---------|-------------|----------|---------|
| `main` | Production-ready code | - | - | `main` |
| `develop` | Integration branch | `main` | `main` | `develop` |
| `feature/*` | New features | `develop` | `develop` | `feature/description` |
| `hotfix/*` | Critical fixes | `main` | `main` + `develop` | `hotfix/description` |
| `release/*` | Release preparation | `develop` | `main` + `develop` | `release/vX.Y.Z` |
| `bugfix/*` | Bug fixes | `develop` | `develop` | `bugfix/description` |

## 📝 Commit Strategy

### Conventional Commits

We use [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Commit Types

| Type | Description | Example |
|------|-------------|---------|
| `feat` | New feature | `feat(auth): add OAuth2 support` |
| `fix` | Bug fix | `fix(install): resolve dependency conflicts` |
| `docs` | Documentation | `docs: update installation guide` |
| `style` | Code style (no logic change) | `style: format with gofmt` |
| `refactor` | Code refactoring | `refactor(config): simplify validation logic` |
| `perf` | Performance improvement | `perf(search): optimize package indexing` |
| `test` | Add/update tests | `test(pack): add integration tests` |
| `chore` | Maintenance tasks | `chore(deps): update dependencies` |
| `ci` | CI/CD changes | `ci: add security scanning` |
| `build` | Build system changes | `build: update Go version to 1.21` |

## 🏷️ Tagging Strategy

### Semantic Versioning

We follow [Semantic Versioning 2.0.0](https://semver.org/):

```
vMAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
```

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Automated Release Process

Use the provided scripts for consistent releases:

```bash
# Create patch release (bug fixes)
make release-patch

# Create minor release (new features)
make release-minor

# Create major release (breaking changes)
make release-major
```

## 🚀 CI/CD Pipeline Features

### Continuous Integration (CI)

**Triggers**: Push to `main`, `develop`, `feature/*`, `hotfix/*`, `release/*` branches

**Pipeline includes**:
- ✅ Multi-platform testing (Linux, macOS, Windows)
- ✅ Multi-Go version testing (1.21, 1.22)
- ✅ Code formatting validation
- ✅ Linting with golangci-lint
- ✅ Security scanning with gosec
- ✅ Vulnerability scanning with Trivy
- ✅ Dependency review for PRs
- ✅ Code quality analysis with CodeQL
- ✅ Test coverage reporting

### Release Pipeline (CD)

**Triggers**: Tags matching `v*` pattern

**Pipeline includes**:
- ✅ Full test suite execution
- ✅ Multi-platform binary builds
- ✅ Security scanning of binaries
- ✅ Automated changelog generation
- ✅ GitHub release creation
- ✅ Docker image building and publishing
- ✅ Checksum generation for binaries

### Security Features

- ✅ **CodeQL analysis** for security vulnerabilities
- ✅ **Dependency scanning** for known vulnerabilities
- ✅ **Secret scanning** to prevent credential leaks
- ✅ **Binary security scanning** before release
- ✅ **Automated dependency updates** with Dependabot

## 🛠️ Development Workflow

### Setup Development Environment

```bash
# Clone and setup
git clone https://github.com/gpm-sh/gpm-cli.git
cd gpm-cli

# Install Git hooks for quality enforcement
make setup-hooks

# Install dependencies and test
make deps
make test
```

### Feature Development

```bash
# 1. Start from develop
git checkout develop
git pull origin develop

# 2. Create feature branch
git checkout -b feature/add-workspace-support

# 3. Develop with quality checks (enforced by hooks)
git add .
git commit -m "feat(install): add workspace configuration parsing"

# 4. Push and create PR
git push origin feature/add-workspace-support
```

### Git Hooks Enforce Quality

Our Git hooks automatically enforce:
- **Code formatting** with gofmt
- **Linting** with golangci-lint
- **Test execution** before commits
- **Commit message validation** (conventional commits)
- **Security checks** before push

## 📊 Release Automation

### Automated Version Management

The release pipeline automatically:

1. **Calculates next version** based on commit types
2. **Updates version references** in code
3. **Generates changelog** from conventional commits
4. **Creates GitHub release** with binaries
5. **Builds Docker images** for deployment
6. **Publishes artifacts** to registries

### Release Types and Triggers

| Release Type | Trigger | Version Bump | Example |
|--------------|---------|--------------|---------|
| Patch | `fix:` commits | 1.0.0 → 1.0.1 | Bug fixes |
| Minor | `feat:` commits | 1.0.0 → 1.1.0 | New features |
| Major | `BREAKING CHANGE:` | 1.0.0 → 2.0.0 | Breaking changes |

### Manual Release Process

```bash
# Create and push a tag to trigger release
git tag -a v1.2.0 -m "Release version 1.2.0"
git push origin v1.2.0

# Or use automated scripts
make release-minor  # Creates tag and pushes automatically
```

## 🔒 Security and Compliance

### Branch Protection

- **Main branch**: Requires PR reviews, status checks, up-to-date branches
- **Develop branch**: Requires PR reviews and status checks
- **No direct pushes** to protected branches
- **Admins included** in restrictions

### Security Scanning

- **SAST** (Static Application Security Testing) with CodeQL
- **Dependency scanning** for vulnerabilities
- **Container scanning** for Docker images
- **Secret detection** in commits and PRs

### Compliance Features

- **Audit trails** for all changes
- **Signed commits** support
- **Reproducible builds** with pinned dependencies
- **SBOM generation** for releases

## 📈 Monitoring and Metrics

### Automated Reporting

- **Test coverage** reports uploaded to Codecov
- **Security findings** in GitHub Security tab
- **Build status** badges in README
- **Performance metrics** from CI runs

### Quality Gates

- **Minimum 80% test coverage** for new code
- **Zero critical security vulnerabilities**
- **All CI checks must pass** before merge
- **Code review approval** required

## 🆘 Troubleshooting

### Common CI/CD Issues

**Tests failing in CI but passing locally:**
```bash
# Run tests in same environment as CI
docker run --rm -v "$PWD":/app -w /app golang:1.21 make test
```

**Build failures:**
```bash
# Check build in CI environment
make build-all  # Test all platforms locally
```

**Security scan failures:**
```bash
# Run security scan locally
make security-scan  # If implemented
```

### Release Issues

**Failed release:**
```bash
# Check GitHub Actions logs
# Fix issues and re-tag
git tag -d v1.0.0
git push origin :refs/tags/v1.0.0
# Fix issues, then re-tag
```

## 📋 Quick Reference

### Essential Commands

```bash
# Development
make setup-hooks       # Install Git hooks
make test             # Run all tests
make build            # Build binary
make lint             # Run linter

# Release
make release-patch    # Patch release
make release-minor    # Minor release
make release-major    # Major release

# Quality
make test-coverage    # Generate coverage report
make security-scan    # Run security checks
```

### Commit Message Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Branch Naming

```bash
feature/add-workspace-support
bugfix/fix-timeout-issue
hotfix/security-vulnerability
release/v1.2.0
```

This comprehensive Git workflow ensures **security, quality, and automation** while maintaining professional development standards and enabling safe, reliable releases.