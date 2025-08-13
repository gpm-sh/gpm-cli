#!/bin/bash

# GPM CLI Release Script
# Automates the release process following Git Flow

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
    exit 1
}

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    error "Not in a git repository"
fi

# Check if git is clean
if [[ -n $(git status --porcelain) ]]; then
    error "Working directory is not clean. Please commit or stash changes."
fi

# Parse command line arguments
RELEASE_TYPE=""
DRY_RUN=false
SKIP_TESTS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        major|minor|patch)
            RELEASE_TYPE="$1"
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [major|minor|patch] [--dry-run] [--skip-tests]"
            echo ""
            echo "Arguments:"
            echo "  major     Create a major release (breaking changes)"
            echo "  minor     Create a minor release (new features)"
            echo "  patch     Create a patch release (bug fixes)"
            echo ""
            echo "Options:"
            echo "  --dry-run     Show what would be done without making changes"
            echo "  --skip-tests  Skip running tests (not recommended)"
            echo "  -h, --help    Show this help message"
            exit 0
            ;;
        *)
            error "Unknown option: $1. Use --help for usage information."
            ;;
    esac
done

if [[ -z "$RELEASE_TYPE" ]]; then
    error "Release type is required. Use 'major', 'minor', or 'patch'."
fi

# Get current version from git tags
CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")
info "Current version: v$CURRENT_VERSION"

# Calculate new version
IFS='.' read -ra VERSION_PARTS <<< "$CURRENT_VERSION"
MAJOR=${VERSION_PARTS[0]}
MINOR=${VERSION_PARTS[1]}
PATCH=${VERSION_PARTS[2]}

case $RELEASE_TYPE in
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    patch)
        PATCH=$((PATCH + 1))
        ;;
esac

NEW_VERSION="$MAJOR.$MINOR.$PATCH"
NEW_TAG="v$NEW_VERSION"

info "New version will be: $NEW_TAG"

if [[ "$DRY_RUN" == true ]]; then
    warning "DRY RUN MODE - No changes will be made"
fi

# Confirm release
if [[ "$DRY_RUN" == false ]]; then
    echo -n "Do you want to create release $NEW_TAG? (y/N) "
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        info "Release cancelled"
        exit 0
    fi
fi

# Check if we're on develop branch
CURRENT_BRANCH=$(git branch --show-current)
if [[ "$CURRENT_BRANCH" != "develop" ]]; then
    warning "Not on develop branch (currently on: $CURRENT_BRANCH)"
    if [[ "$DRY_RUN" == false ]]; then
        echo -n "Switch to develop branch? (y/N) "
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            git checkout develop
            git pull origin develop
        else
            error "Please switch to develop branch before releasing"
        fi
    fi
fi

# Run tests
if [[ "$SKIP_TESTS" == false ]]; then
    info "Running tests..."
    if [[ "$DRY_RUN" == false ]]; then
        make test || error "Tests failed"
    else
        info "Would run: make test"
    fi
    success "Tests passed"
fi

# Create release branch
RELEASE_BRANCH="release/v$NEW_VERSION"
info "Creating release branch: $RELEASE_BRANCH"

if [[ "$DRY_RUN" == false ]]; then
    git checkout -b "$RELEASE_BRANCH"
else
    info "Would run: git checkout -b $RELEASE_BRANCH"
fi

# Update version in files (if any version files exist)
info "Updating version references..."
if [[ "$DRY_RUN" == false ]]; then
    # Update version in main.go if it exists
    if grep -q "Version.*=" main.go 2>/dev/null; then
        sed -i.bak "s/Version.*=.*/Version = \"$NEW_VERSION\"/" main.go
        rm -f main.go.bak
        git add main.go
    fi
    
    # Commit version bump
    git commit -m "chore(release): bump version to v$NEW_VERSION" || info "No version files to update"
else
    info "Would update version references and commit changes"
fi

# Build and test the release
info "Building release..."
if [[ "$DRY_RUN" == false ]]; then
    make build || error "Build failed"
    make test || error "Tests failed after version bump"
else
    info "Would run: make build && make test"
fi

# Merge to main
info "Merging to main branch..."
if [[ "$DRY_RUN" == false ]]; then
    git checkout main
    git pull origin main
    git merge --no-ff "$RELEASE_BRANCH" -m "Release v$NEW_VERSION"
else
    info "Would run: git checkout main && git merge --no-ff $RELEASE_BRANCH"
fi

# Create and push tag
info "Creating tag: $NEW_TAG"
if [[ "$DRY_RUN" == false ]]; then
    git tag -a "$NEW_TAG" -m "Release version $NEW_VERSION"
    git push origin main
    git push origin "$NEW_TAG"
else
    info "Would run: git tag -a $NEW_TAG && git push origin main --tags"
fi

# Merge back to develop
info "Merging back to develop..."
if [[ "$DRY_RUN" == false ]]; then
    git checkout develop
    git merge --no-ff main -m "Merge release v$NEW_VERSION back to develop"
    git push origin develop
else
    info "Would run: git checkout develop && git merge --no-ff main"
fi

# Clean up release branch
info "Cleaning up release branch..."
if [[ "$DRY_RUN" == false ]]; then
    git branch -d "$RELEASE_BRANCH"
    git push origin --delete "$RELEASE_BRANCH" 2>/dev/null || info "Release branch not on remote"
else
    info "Would delete release branch: $RELEASE_BRANCH"
fi

success "Release $NEW_TAG completed successfully!"

if [[ "$DRY_RUN" == false ]]; then
    info "GitHub Actions will automatically:"
    info "  • Run tests and build binaries"
    info "  • Create GitHub release with changelog"
    info "  • Build and push Docker image"
    info ""
    info "Monitor the release at: https://github.com/$(git config --get remote.origin.url | sed 's/.*://; s/.git$//')/actions"
else
    info "This was a dry run. No changes were made."
fi
