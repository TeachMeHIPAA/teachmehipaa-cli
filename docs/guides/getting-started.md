# Getting started

This guide takes you from nothing to inviting your first Learner in the
Sandbox Environment with `tmh`. It takes about ten minutes.

## Install

The Developer API is in pre-release and `tmh` has no published release yet,
so install it from source. With Go 1.24 or newer:

<!-- doctest: illustrative -->
```sh
go install github.com/TeachMeHIPAA/teachmehipaa-cli/cmd/tmh@main
```

That puts `tmh` in `$(go env GOPATH)/bin`, which should be on your `PATH`.
Or build it from a checkout, which puts it in `bin/tmh`:

<!-- doctest: illustrative -->
```sh
git clone https://github.com/TeachMeHIPAA/teachmehipaa-cli.git
cd teachmehipaa-cli
make build
```

A build from source reports its version as `dev`; a release build reports
its version, commit and build date:

<!-- doctest: version -->
```console
$ tmh version
tmh version 1.0.0 (commit 39276f1, built 2026-11-15T19:27:47Z)
```

### After the first release

These channels work once `tmh` is released:

| Platform | Command |
|---|---|
| macOS, Linux (Homebrew) | `brew install --cask teachmehipaa/tap/tmh` |
| Windows (Scoop) | `scoop bucket add teachmehipaa https://github.com/TeachMeHIPAA/scoop-bucket`<br>`scoop install teachmehipaa/tmh` |
| Debian, Ubuntu | download `tmh_<version>_<arch>.deb` from [Releases](https://github.com/TeachMeHIPAA/teachmehipaa-cli/releases), then `sudo dpkg -i tmh_<version>_amd64.deb` |
| Fedora, RHEL | download `tmh-<version>-1.<arch>.rpm` from Releases, then `sudo rpm -i tmh-<version>-1.x86_64.rpm` |
| Docker | `docker run --rm ghcr.io/teachmehipaa/tmh version` |
| Go 1.24+ | `go install github.com/TeachMeHIPAA/teachmehipaa-cli/cmd/tmh@latest` |
| Anything else | download the archive for your OS and CPU from Releases and put `tmh` on your `PATH` |

Release archives are `.tar.gz` (`.zip` on Windows) for macOS, Linux and
Windows on amd64 and arm64. Each holds `tmh`, the README, the license, the
changelog and completion scripts. The deb and rpm packages install the
completions for bash, zsh and fish too.

`checksums.txt` lists the SHA-256 of every archive and package. Each release
also has `checksums.txt.sigstore.json`, a keyless Sigstore signature made by
the release workflow. To verify a download:

<!-- doctest: illustrative -->
```sh
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/TeachMeHIPAA/teachmehipaa-cli/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --check --ignore-missing checksums.txt
```

The Docker image runs as a non-root user and has no OS keyring, so pass
credentials as environment variables. Mount a volume at
`/home/nonroot/.config/tmh` to keep `tmh`'s
[state files](configuration-and-privacy.md#configuration-and-state-files)
between runs:

<!-- doctest: illustrative -->
```sh
docker run --rm -e TMH_CLIENT_ID -e TMH_CLIENT_SECRET \
  ghcr.io/teachmehipaa/tmh learners list --limit 5
```

## Get Sandbox credentials

An Organization admin creates an **API client** in the TeachMeHIPAA console;
`tmh open console` opens it. An API client has a client id (`cli_…`) and a
client secret (`tmh_cs_…`), and belongs to exactly one Organization and one
Environment. Create it in the Sandbox first: Sandbox Learners are never
emailed, so you can experiment freely.

## Log in

`tmh login` asks for the id and secret, checks them with TeachMeHIPAA, and
stores them: the secret in your OS keyring, the id in `tmh`'s config file.

<!-- doctest: illustrative -->
```console
$ tmh login
Client ID: cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
Client secret:
Logged in to Sandbox as Organization org_acme.
```

The secret isn't echoed as you type it. `tmh whoami` shows what the stored
credentials can do:

<!-- doctest: whoami -->
```console
$ tmh whoami
Organization  org_acme
Scopes        learners:read, learners:write
Environment   Sandbox
```

In CI, or anywhere you'd rather not store credentials, set
`TMH_CLIENT_ID` and `TMH_CLIENT_SECRET` instead; `tmh` then needs no
`login`. [Authentication and profiles](authentication-and-profiles.md)
covers both, and the `live` profile.

## Make your first calls

Every API operation is a command named `tmh <resource> <method>`. The
Compliance summary counts your Learners by status:

<!-- doctest: first-call -->
```console
$ tmh compliance retrieve
{
  "asOf": "2026-11-15T12:00:00.000Z",
  "courseId": "hipaa-2026",
  "totals": {
    "learners": 42,
    "compliant": 30,
    "expiringSoon": 3,
    "expired": 1,
    "inProgress": 5,
    "notStarted": 2,
    "inviteBounced": 1
  },
  "compliantRate": 0.714
}
```

`tmh` fetched an Access token with your credentials, asked the API which
Organization they belong to, and filled it into the request path. You never
pass an Organization id or handle a token.

`tmh <resource> --help` lists a resource's methods, and
`tmh <resource> <method> --help` shows the API's description of one, its
Scopes and its flags:

<!-- doctest: illustrative -->
```sh
tmh learners --help
tmh learners list --help
```

## Invite a Learner

`tmh learners create` invites someone to a Course by email. Body fields are
flags:

<!-- doctest: invite -->
```console
$ tmh learners create --email ben@example.com --name "Ben Example"
{
  "learnerId": "sub_01JD8A7Y3K2M4N5P6Q7R8S9T0V",
  "email": "ben@example.com",
  "name": "Ben Example",
  "externalId": null,
  "status": "not_started",
  "course": {
    "courseId": "hipaa-2026",
    "title": "HIPAA Training 2026"
  },
  "invitedAt": "2026-11-15T12:00:00.000Z",
  "startedAt": null,
  "completedAt": null,
  "dueAt": "2026-12-15T00:00:00.000Z",
  "certificate": null,
  "delivery": "email"
}
```

If you sync people from another system, prefer
`tmh learners upsert-by-external-id`, keyed by your own id for the person,
their External ID. It creates and invites the Learner the first time and
returns them every time after, so a sync job can replay it safely:

<!-- doctest: illustrative -->
```sh
tmh learners upsert-by-external-id hr-124 --email ben@example.com --name "Ben Example"
```

The [roster sync example](../../examples/sync-roster.sh) does that for every
row of a CSV file.

## Handle failures

A failed command prints the API's problem and its request id on stderr, and
exits with a code for the kind of failure:

<!-- doctest: getting-started-not-found -->
```console
$ tmh learners retrieve sub_01JD8ZZZZZZZZZZZZZZZZZZZZZ
Error: Not found: No Learner with that id.
Request ID: req_01JD8Q
$ echo $?
3
```

[Exit codes and errors](exit-codes.md) lists every code.

## Next steps

- [Commands](commands.md): arguments, flags, `-d`, ETags and raw requests.
- [Output and scripting](output-and-scripting.md): `jq`, `--all` and
  `--ndjson`.
- [Webhooks and `events tail`](webhooks-and-events-tail.md): build your
  webhook handler against real Events on your machine.
- The [command reference](../reference/commands/tmh.md) and the
  [resource reference](../reference/resources.md).
