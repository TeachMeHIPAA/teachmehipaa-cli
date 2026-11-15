#!/bin/sh
# Writes tmh's shell completion scripts into completions/ for the release
# archives and packages. GoReleaser runs it before building.
set -eu
rm -rf completions
mkdir completions
go build -o completions/.tmh ./cmd/tmh
for shell in bash zsh fish; do
	completions/.tmh completion "$shell" >"completions/tmh.$shell"
done
completions/.tmh completion powershell >completions/tmh.ps1
rm completions/.tmh
