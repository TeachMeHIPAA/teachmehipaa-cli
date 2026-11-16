# Install

<!-- doctest: synopsis -->
```text
tmh version
tmh completion bash
tmh completion zsh
tmh completion fish
tmh completion powershell
```

## From source

`tmh` has no tagged release while the Developer API is in pre-release, so
source is the only channel today. With Go 1.24 or newer:

<!-- doctest: illustrative -->
```sh
go install github.com/TeachMeHIPAA/teachmehipaa-cli/cmd/tmh@main
```

The binary goes to `$(go env GOPATH)/bin`; put that directory on your
`PATH`. From a checkout, `make build` writes `bin/tmh`:

<!-- doctest: illustrative -->
```sh
git clone https://github.com/TeachMeHIPAA/teachmehipaa-cli.git
cd teachmehipaa-cli
make build
```

A source build prints `dev` as its version. A release build prints the
version, the commit and the build time:

<!-- doctest: version -->
```console
$ tmh version
tmh version 1.0.0 (commit 39276f1, built 2026-11-15T19:27:47Z)
```

## Release channels

These go live with the first tagged release:

| Platform | Command |
|---|---|
| macOS, Linux (Homebrew) | `brew install --cask teachmehipaa/tap/tmh` |
| Windows (Scoop) | `scoop bucket add teachmehipaa https://github.com/TeachMeHIPAA/scoop-bucket`<br>`scoop install teachmehipaa/tmh` |
| Debian, Ubuntu | download `tmh_<version>_<arch>.deb` from [Releases](https://github.com/TeachMeHIPAA/teachmehipaa-cli/releases), then `sudo dpkg -i tmh_<version>_amd64.deb` |
| Fedora, RHEL | download `tmh-<version>-1.<arch>.rpm` from Releases, then `sudo rpm -i tmh-<version>-1.x86_64.rpm` |
| Docker | `docker run --rm ghcr.io/teachmehipaa/tmh version` |
| Go 1.24+ | `go install github.com/TeachMeHIPAA/teachmehipaa-cli/cmd/tmh@latest` |
| Anything else | the archive for your OS and CPU from Releases, with `tmh` on your `PATH` |

Archives are `.tar.gz` (`.zip` on Windows) for macOS, Linux and Windows on
amd64 and arm64. Each one holds `tmh`, the README, the license, the
changelog and completion scripts. The deb and rpm packages also install the
bash, zsh and fish completions.

## Verify a download

`checksums.txt` lists the SHA-256 of every archive and package.
`checksums.txt.sigstore.json` is a keyless Sigstore signature over it, made
by the release workflow:

<!-- doctest: illustrative -->
```sh
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/TeachMeHIPAA/teachmehipaa-cli/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --check --ignore-missing checksums.txt
```

## Docker

<!-- doctest: illustrative -->
```sh
docker run --rm -e TMH_CLIENT_ID -e TMH_CLIENT_SECRET \
  ghcr.io/teachmehipaa/tmh learners list --limit 5
```

The image is distroless, runs as a non-root user and has no OS keyring, so
pass credentials as environment variables. Mount a volume at
`/home/nonroot/.config/tmh` to keep the
[state files](environment.md#configuration-and-state-files) between runs.

## Shell completion

<!-- doctest: illustrative -->
```sh
source <(tmh completion bash)    # in ~/.bashrc
source <(tmh completion zsh)     # in ~/.zshrc
tmh completion fish | source     # in ~/.config/fish/config.fish
```

In PowerShell, add `tmh completion powershell | Out-String | Invoke-Expression`
to `$PROFILE`. `tmh completion <shell> --help` shows how to install the
script for good. Completion offers resources, methods, flags and enum
values. Homebrew and the deb and rpm packages install it for bash, zsh and
fish.

## Next

[login](login.md) to store credentials, then try a [recipe](recipes.md).
