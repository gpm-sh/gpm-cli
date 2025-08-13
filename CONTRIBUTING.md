# Contributing to GPM CLI

Thank you for your interest in contributing to GPM CLI! This document provides guidelines and information for contributors.

## 🚀 Quick Start

1. **Fork the repository** and clone your fork
2. **Create a feature branch** from `develop`
3. **Make your changes** following our coding standards
4. **Run tests** to ensure everything works
5. **Submit a pull request** with a clear description

## 🌳 Branching Strategy

We follow **Git Flow** branching model:

### Main Branches

- **`main`** - Production-ready code, tagged releases only
- **`develop`** - Integration branch for features, default branch

### Supporting Branches

- **`feature/*`** - New features and enhancements
- **`hotfix/*`** - Critical fixes for production
- **`release/*`** - Preparation for releases
- **`bugfix/*`** - Bug fixes for develop branch

### Branch Naming Convention

```
feature/add-workspace-support
feature/improve-error-handling
bugfix/fix-login-timeout
hotfix/security-vulnerability-fix
release/v1.2.0
```

## 🏷️ Commit Strategy

### Conventional Commits

We use [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Commit Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, etc.)
- **refactor**: Code refactoring
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **chore**: Maintenance tasks
- **ci**: CI/CD changes
- **build**: Build system changes

### Examples

```bash
# Good commit messages
feat(install): add workspace support for monorepos
fix(auth): resolve token encryption issue on Windows
docs: update installation instructions
test(pack): add integration tests for tarball creation
chore(deps): update Go dependencies

# Bad commit messages
update stuff
fix bug
improvements
```

### Commit Guidelines

- **Use present tense**: "add feature" not "added feature"
- **Keep subject line under 50 characters**
- **Use body to explain what and why, not how**
- **Reference issues**: `Closes #123`, `Fixes #456`

## 🏷️ Tagging Strategy

### Version Format

We follow [Semantic Versioning](https://semver.org/):

```
vMAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
```

### Tag Types

- **Release tags**: `v1.0.0`, `v1.2.3`
- **Pre-release tags**: `v1.0.0-beta.1`, `v2.0.0-rc.1`
- **Alpha tags**: `v1.0.0-alpha.1`

### Creating Releases

1. Create release branch from `develop`:
   ```bash
   git checkout develop
   git pull origin develop
   git checkout -b release/v1.2.0
   ```

2. Update version in files:
   - Update version references
   - Update CHANGELOG.md
   - Commit changes

3. Merge to main and tag:
   ```bash
   git checkout main
   git merge --no-ff release/v1.2.0
   git tag -a v1.2.0 -m "Release version 1.2.0"
   git push origin main --tags
   ```

4. Merge back to develop:
   ```bash
   git checkout develop
   git merge --no-ff main
   git push origin develop
   ```

## 🔄 Workflow Process

### Feature Development

```bash
# 1. Start from develop
git checkout develop
git pull origin develop

# 2. Create feature branch
git checkout -b feature/my-new-feature

# 3. Make changes and commit
git add .
git commit -m "feat: add my new feature"

# 4. Push and create PR
git push origin feature/my-new-feature
# Create PR to develop branch
```

### Hotfix Process

```bash
# 1. Start from main
git checkout main
git pull origin main

# 2. Create hotfix branch
git checkout -b hotfix/critical-fix

# 3. Make fix and commit
git add .
git commit -m "fix: resolve critical security issue"

# 4. Push and create PR to main
git push origin hotfix/critical-fix
# Create PR to main branch
```

## 🧪 Testing Requirements

### Before Submitting PR

```bash
# Run all tests
make test

# Check code quality
make lint
make fmt

# Build and smoke test
make build
./gpm version
./gpm help
```

### Test Coverage

- **Minimum 80% test coverage** for new code
- **Unit tests** for all new functions
- **Integration tests** for new commands
- **Error scenarios** must be tested

### Test Categories

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test command workflows
3. **End-to-End Tests**: Test complete user scenarios

## 📝 Code Style

### Go Standards

- Follow **Go best practices** and idioms
- Use **gofmt** for formatting
- Follow **golangci-lint** recommendations
- **No hardcoded values** - use configuration
- **Proper error handling** with context

### CLI Standards

- **Consistent command structure** with Cobra
- **Professional styling** using our styling package
- **Helpful error messages** with recovery hints
- **Input validation** for all user inputs

### Documentation

- **Go doc comments** for all exported functions
- **Command help text** for all commands
- **README updates** for user-facing changes
- **Inline comments** for complex logic only

## 🔒 Security Guidelines

### Secure Coding

- **Validate all inputs** before processing
- **Sanitize user data** to prevent injection
- **Use secure random** for cryptographic operations
- **Clear sensitive data** from memory after use
- **Handle credentials securely** (no plaintext storage)

### Dependencies

- **Keep dependencies minimal** and up-to-date
- **Audit dependencies** for vulnerabilities
- **Pin dependency versions** in go.mod
- **Review dependency licenses** for compatibility

## 📋 Pull Request Process

### Before Creating PR

1. **Rebase on target branch** to avoid merge conflicts
2. **Run all tests** and ensure they pass
3. **Update documentation** if needed
4. **Add/update tests** for your changes
5. **Follow commit message format**

### PR Requirements

- **Clear title and description**
- **Link to related issues**
- **Explain the changes** and their impact
- **Include test instructions**
- **Screenshots/demos** for UI changes

### Review Process

1. **Automated checks** must pass (CI/CD)
2. **Code review** by maintainers
3. **Security review** for sensitive changes
4. **Manual testing** by reviewers
5. **Approval** from code owners

## 🐛 Bug Reports

### Before Reporting

1. **Search existing issues** to avoid duplicates
2. **Try latest version** to see if already fixed
3. **Gather system information** and logs
4. **Create minimal reproduction** case

### Good Bug Reports Include

- **Clear description** of the problem
- **Steps to reproduce** the issue
- **Expected vs actual behavior**
- **System information** (OS, Go version, etc.)
- **Command output** and error messages

## 💡 Feature Requests

### Before Requesting

1. **Check existing issues** and roadmap
2. **Consider if it fits** the project scope
3. **Think about backwards compatibility**
4. **Consider maintenance burden**

### Good Feature Requests Include

- **Clear problem statement**
- **Proposed solution** with examples
- **Use cases** and benefits
- **Implementation ideas** (if any)

## 🆘 Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and ideas
- **Documentation**: Check README and inline docs
- **Code Examples**: Look at existing commands

## 📄 License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

---

## Thank You! 🙏

Your contributions help make GPM CLI better for everyone. We appreciate your time and effort!
