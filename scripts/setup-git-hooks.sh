#!/bin/bash

# Setup Git hooks for GPM CLI development
# This script installs Git hooks that enforce code quality and consistency

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

echo "🔧 Setting up Git hooks for GPM CLI..."

# Create hooks directory if it doesn't exist
mkdir -p "$HOOKS_DIR"

# Pre-commit hook
cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/bin/bash

# GPM CLI Pre-commit hook
# This hook runs before each commit to ensure code quality

set -e

echo "🔍 Running pre-commit checks..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed or not in PATH"
    exit 1
fi

# Format code
echo "📝 Formatting code..."
make fmt

# Check for changes after formatting
if ! git diff --exit-code; then
    echo "❌ Code formatting changes detected. Please stage the formatted files and commit again."
    exit 1
fi

# Run linter
echo "🔍 Running linter..."
if ! make lint; then
    echo "❌ Linter issues found. Please fix them before committing."
    exit 1
fi

# Run tests
echo "🧪 Running tests..."
if ! make test-unit; then
    echo "❌ Tests failed. Please fix them before committing."
    exit 1
fi

# Check for TODO/FIXME comments in staged files
echo "📋 Checking for TODO/FIXME comments..."
staged_files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)
if [ -n "$staged_files" ]; then
    todo_count=$(grep -n "TODO\|FIXME\|XXX\|HACK" $staged_files | wc -l || true)
    if [ "$todo_count" -gt 0 ]; then
        echo "⚠️  Found $todo_count TODO/FIXME comments in staged files:"
        grep -n "TODO\|FIXME\|XXX\|HACK" $staged_files || true
        echo "Consider addressing these before committing."
    fi
fi

echo "✅ Pre-commit checks passed!"
EOF

# Commit message hook
cat > "$HOOKS_DIR/commit-msg" << 'EOF'
#!/bin/bash

# GPM CLI Commit message hook
# This hook validates commit messages against conventional commit format

commit_regex='^(feat|fix|docs|style|refactor|perf|test|chore|ci|build)(\(.+\))?: .{1,50}'

error_msg="❌ Invalid commit message format!

Commit message should follow conventional commits format:
<type>[optional scope]: <description>

Examples:
  feat: add workspace support
  fix(auth): resolve token encryption issue
  docs: update installation guide
  test(pack): add integration tests

Types: feat, fix, docs, style, refactor, perf, test, chore, ci, build
"

if ! grep -qE "$commit_regex" "$1"; then
    echo "$error_msg" >&2
    exit 1
fi

# Check commit message length
if [ $(head -n1 "$1" | wc -c) -gt 72 ]; then
    echo "❌ Commit message subject line too long (max 72 characters)" >&2
    exit 1
fi

echo "✅ Commit message format is valid!"
EOF

# Pre-push hook
cat > "$HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash

# GPM CLI Pre-push hook
# This hook runs before pushing to remote repository

set -e

echo "🚀 Running pre-push checks..."

# Get the remote and URL
remote="$1"
url="$2"

# Run full test suite
echo "🧪 Running full test suite..."
if ! make test; then
    echo "❌ Tests failed. Push aborted."
    exit 1
fi

# Check for sensitive information
echo "🔍 Checking for sensitive information..."
if git log --oneline | grep -i "password\|secret\|key\|token" | head -5; then
    echo "⚠️  Found potentially sensitive information in recent commits."
    echo "Please review your commits before pushing."
fi

# Build to ensure everything compiles
echo "🔨 Building project..."
if ! make build; then
    echo "❌ Build failed. Push aborted."
    exit 1
fi

echo "✅ Pre-push checks passed!"
EOF

# Make hooks executable
chmod +x "$HOOKS_DIR/pre-commit"
chmod +x "$HOOKS_DIR/commit-msg"
chmod +x "$HOOKS_DIR/pre-push"

# Create prepare-commit-msg hook for conventional commits help
cat > "$HOOKS_DIR/prepare-commit-msg" << 'EOF'
#!/bin/bash

# GPM CLI Prepare commit message hook
# This hook adds a template for conventional commits

# Only add template for new commits (not amends, merges, etc.)
case "$2,$3" in
  ,|template,)
    # Add conventional commit template if message is empty
    if [ ! -s "$1" ]; then
        echo "# Enter commit message following conventional commits format:" >> "$1"
        echo "# <type>[optional scope]: <description>" >> "$1"
        echo "#" >> "$1"
        echo "# Types: feat, fix, docs, style, refactor, perf, test, chore, ci, build" >> "$1"
        echo "# Examples:" >> "$1"
        echo "#   feat: add new command" >> "$1"
        echo "#   fix(auth): resolve login issue" >> "$1"
        echo "#   docs: update README" >> "$1"
        echo "" >> "$1"
    fi
    ;;
  *) ;;
esac
EOF

chmod +x "$HOOKS_DIR/prepare-commit-msg"

echo "✅ Git hooks installed successfully!"
echo ""
echo "📋 Installed hooks:"
echo "  • pre-commit: Format, lint, and test code"
echo "  • commit-msg: Validate commit message format"
echo "  • pre-push: Run full test suite before push"
echo "  • prepare-commit-msg: Add conventional commit template"
echo ""
echo "💡 To skip hooks temporarily, use: git commit --no-verify"
echo "🔧 To update hooks, run this script again"
