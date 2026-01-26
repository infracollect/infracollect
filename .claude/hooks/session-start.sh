#!/bin/bash
# Session start hook: Install mise and project dependencies
set -e

MISE_VERSION="v2026.1.7"
MISE_INSTALL_DIR="$HOME/.local/bin"
MISE_BIN="$MISE_INSTALL_DIR/mise"

# Create install directory if it doesn't exist
mkdir -p "$MISE_INSTALL_DIR"

# Download and install mise if not present or version mismatch
install_mise() {
    local tmp_dir
    tmp_dir=$(mktemp -d)
    local tarball="$tmp_dir/mise.tar.gz"
    local url="https://github.com/jdx/mise/releases/download/${MISE_VERSION}/mise-${MISE_VERSION}-linux-x64.tar.gz"

    echo "Downloading mise ${MISE_VERSION}..."
    if ! curl -fsSL -o "$tarball" "$url"; then
        echo "Failed to download mise" >&2
        rm -rf "$tmp_dir"
        return 1
    fi

    echo "Extracting mise..."
    tar -xzf "$tarball" -C "$tmp_dir"
    cp "$tmp_dir/mise/bin/mise" "$MISE_BIN"
    chmod +x "$MISE_BIN"

    rm -rf "$tmp_dir"
    echo "mise ${MISE_VERSION} installed to $MISE_BIN"
}

# Check if mise needs to be installed or updated
if [ ! -x "$MISE_BIN" ]; then
    echo "mise not found, installing..."
    install_mise
else
    current_version=$("$MISE_BIN" --version 2>/dev/null | head -1 | awk '{print $1}')
    expected_version="${MISE_VERSION#v}"
    if [ "$current_version" != "$expected_version" ]; then
        echo "mise version mismatch (have: $current_version, want: $expected_version), updating..."
        install_mise
    else
        echo "mise ${expected_version} already installed"
    fi
fi

# Add mise to PATH for this session
if [ -n "$CLAUDE_ENV_FILE" ]; then
    echo "export PATH=\"$MISE_INSTALL_DIR:\$PATH\"" >> "$CLAUDE_ENV_FILE"
fi

# Trust the project's mise configuration
if [ -f "$CLAUDE_PROJECT_DIR/mise.toml" ]; then
    echo "Trusting mise configuration..."
    "$MISE_BIN" trust "$CLAUDE_PROJECT_DIR/mise.toml" 2>/dev/null || true
fi

# Install mise dependencies
echo "Running mise install..."
"$MISE_BIN" install --yes 2>&1 || true

echo "Session setup complete"
exit 0
