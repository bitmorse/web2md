#!/bin/bash
# install.sh - Install web2md on Linux or macOS

set -e

REPO="bitmorse/web2md"
BINARY_NAME="web2md"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin) OS_NAME="darwin" ;;
  linux)  OS_NAME="linux" ;;
  *)
    echo "Unsupported operating system: $OS"
    exit 1
    ;;
esac

# Detect architecture
case "$(uname -m)" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)"
    exit 1
    ;;
esac

echo "Installing $BINARY_NAME for $OS_NAME/$ARCH..."

DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/${BINARY_NAME}-${OS_NAME}-${ARCH}.tar.gz"
echo "Downloading from $DOWNLOAD_URL"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$BINARY_NAME.tar.gz"
tar -xzf "$TMP_DIR/$BINARY_NAME.tar.gz" -C "$TMP_DIR"

EXTRACTED="$TMP_DIR/${BINARY_NAME}-${OS_NAME}-${ARCH}"
if [ ! -f "$EXTRACTED" ]; then
  echo "Error: expected binary not found after extraction"
  exit 1
fi
chmod +x "$EXTRACTED"

# Try install locations in order
INSTALL_PATH=""

# 1. /usr/local/bin with sudo
if command -v sudo >/dev/null 2>&1; then
  if sudo install -m 755 "$EXTRACTED" /usr/local/bin/$BINARY_NAME 2>/dev/null; then
    INSTALL_PATH="/usr/local/bin/$BINARY_NAME"
  fi
fi

# 2. /usr/local/bin direct
if [ -z "$INSTALL_PATH" ]; then
  if install -m 755 "$EXTRACTED" /usr/local/bin/$BINARY_NAME 2>/dev/null; then
    INSTALL_PATH="/usr/local/bin/$BINARY_NAME"
  fi
fi

# 3. ~/.local/bin
if [ -z "$INSTALL_PATH" ]; then
  mkdir -p "$HOME/.local/bin"
  if mv "$EXTRACTED" "$HOME/.local/bin/$BINARY_NAME"; then
    INSTALL_PATH="$HOME/.local/bin/$BINARY_NAME"
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
      SHELL_RC=""
      [ -f "$HOME/.zshrc" ] && SHELL_RC="$HOME/.zshrc"
      [ -f "$HOME/.bashrc" ] && SHELL_RC="$HOME/.bashrc"
      if [ -n "$SHELL_RC" ]; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_RC"
        echo "Added ~/.local/bin to PATH in $SHELL_RC — restart your shell or run: source $SHELL_RC"
      else
        echo "Add ~/.local/bin to your PATH manually"
      fi
    fi
  fi
fi

if [ -z "$INSTALL_PATH" ]; then
  echo "Error: all installation methods failed"
  exit 1
fi

echo ""
echo "Installed $BINARY_NAME to $INSTALL_PATH"
"$INSTALL_PATH" --help 2>/dev/null | head -3 || true
echo ""
echo "Usage:"
echo "  $BINARY_NAME https://example.com --convert-md"
echo "  $BINARY_NAME search \"query\" -d example.com"
