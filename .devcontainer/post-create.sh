#!/bin/bash
set -euo pipefail
set -x

GO_VERSION=$(go list -m -f '{{.GoVersion}}')
# Resolve latest patch version if only major.minor is specified
if [[ "$GO_VERSION" =~ ^[0-9]+\.[0-9]+$ ]]; then
  GO_VERSION=$(
    curl -fsSL 'https://go.dev/dl/?mode=json&include=all' |
      grep -oE '"version": "go'"$GO_VERSION"'\.[0-9]+"' |
      sed -E 's/.*go([0-9]+\.[0-9]+\.[0-9]+).*/\1/' |
      sort -V |
      tail -1
  )
fi

echo "Installing Go $GO_VERSION"
go install "golang.org/dl/go${GO_VERSION}@latest"
"go${GO_VERSION}" download
sudo ln -sfn "$HOME/sdk/go${GO_VERSION}" /usr/local/go
export PATH="/usr/local/go/bin:$HOME/go/bin:$HOME/.local/bin:$PATH"
go version

# download go modules
go mod download

# build custom golangci-lint
echo "Installing golangci-lint"
mkdir -p "$HOME/.local/bin"
LINT_TMP=$(mktemp -d)
trap 'rm -rf "$LINT_TMP"' EXIT
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "$LINT_TMP/bootstrap" latest
echo "Building custom golangci-lint"
"$LINT_TMP/bootstrap/golangci-lint" custom --destination "$LINT_TMP/build" --name golangci-lint
install -m 755 "$LINT_TMP/build/golangci-lint" "$HOME/.local/bin/.golangci-lint.new"
mv -f "$HOME/.local/bin/.golangci-lint.new" "$HOME/.local/bin/golangci-lint"
"$HOME/.local/bin/golangci-lint" version

# install Go debug packages
sudo apt update
sudo apt install -y graphviz

git config --global --add safe.directory /workspace/xddns
