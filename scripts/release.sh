#!/bin/bash
set -e

# Get current version
VERSION=$(cat VERSION 2>/dev/null || echo "v0.1.0")
DATE=$(date +'%Y-%m-%d')

# Parse arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        -v|--version) VERSION="$2"; shift ;;
        *) echo "Unknown parameter: $1"; exit 1 ;;
    esac
    shift
done

echo "Creating release $VERSION"

# Build binaries for all platforms
make build-all

# Create release notes
echo "# Exchange Connector Release $VERSION" > release_notes.md
echo "Released on $DATE" >> release_notes.md
echo "" >> release_notes.md
echo "## Included builds" >> release_notes.md
echo "- Linux (AMD64)" >> release_notes.md
echo "- Linux (ARM64)" >> release_notes.md
echo "- macOS (AMD64)" >> release_notes.md
echo "- macOS (ARM64/Apple Silicon)" >> release_notes.md

# Check if gh is installed
if ! command -v gh &> /dev/null; then
    echo "GitHub CLI not found. Please install it first."
    exit 1
fi

# Create release using GitHub CLI
gh release create $VERSION \
  --title "Exchange Connector $VERSION" \
  --notes-file release_notes.md \
  build/exchange-connector-linux-amd64 \
  build/exchange-connector-linux-arm64 \
  build/exchange-connector-darwin-amd64 \
  build/exchange-connector-darwin-arm64

echo "Release $VERSION created successfully!" 