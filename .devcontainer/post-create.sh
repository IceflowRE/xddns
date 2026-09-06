#!/bin/bash
set -ex

go install golang.org/dl/go1.27.0@latest
go1.27.0 download
sudo rm -rf /usr/local/go
sudo ln -s /home/vscode/sdk/go1.27.0 /usr/local/go
go version

# download go modules
go mod download

# install golangci-lint and build custom golangci-lint
echo "Installing golangci-lint"
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin
echo "Building custom golangci-lint"
golangci-lint custom --destination "/home/vscode/.local/bin" --name golangci-lint
rm $(go env GOPATH)/bin/golangci-lint

# install Go debug packages
sudo apt update
sudo apt install -y graphviz

git config --global --add safe.directory /workspace/xddns
