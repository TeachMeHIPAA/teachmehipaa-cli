# Authentication and profiles

`tmh` authenticates as an **API client**: a client id (`cli_…`) and client
secret (`tmh_cs_…`) that an Organization admin creates in the TeachMeHIPAA
console. An API client belongs to exactly one Organization and one
Environment, and holds a set of Scopes. `tmh` exchanges it for short-lived
Access tokens itself and keeps them in memory for the life of one command, so
you never handle a token.

## Logging in

`tmh login` asks for the client id and secret, checks them with
TeachMeHIPAA, and only then stores them:

<!-- doctest: illustrative -->
```console
$ tmh login
Client ID: cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
Client secret:
Logged in to Sandbox as Organization org_acme.
```

The secret goes into your OS keyring: the macOS Keychain, the Windows
Credential Manager, or the Secret Service on Linux. Where no keyring works,
as in most CI containers, it goes into `config.toml` instead, readable only
by you, and `tmh` says so on stderr.

Without a terminal, pass the id as a flag and pipe the secret on stdin, so it
never appears in your shell history or process list:

<!-- doctest: login-piped -->
```console
$ printf %s "$TMH_SECRET" | tmh login --client-id cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
Logged in to Sandbox as Organization org_acme.
```

`tmh logout` removes the profile's stored credentials:

<!-- doctest: logout -->
```console
$ tmh logout
Logged out of the sandbox profile.
```

## Profiles and Environments

Credentials belong to one Environment, so `tmh` keeps one profile for each:

| Profile | Environment | API base URL |
|---|---|---|
| `sandbox` (the default) | Sandbox | `https://api-teachmehipaa-dev.web.app/v1` |
| `live` | Live | `https://api.tme.so/v1` |

Every command uses `sandbox` unless you pass `--live` (or `--profile live`),
so you never reach Live by accident. Log in to each profile separately:

<!-- doctest: illustrative -->
```sh
tmh login --live
tmh learners list --live
tmh whoami --profile live
```

A command for a profile you haven't logged in to tells you how to:

<!-- doctest: not-logged-in -->
```console
$ tmh compliance retrieve --live
Error: not logged in to the live profile; run `tmh login --live` or set TMH_CLIENT_ID and TMH_CLIENT_SECRET
$ echo $?
2
```

## Credentials from the environment

Set `TMH_CLIENT_ID` and `TMH_CLIENT_SECRET` to use those credentials instead
of stored ones. CI then needs no `tmh login` and no keyring. Set both or
neither. The profile still picks the Environment, so pass `--live` for Live
credentials:

<!-- doctest: env-credentials -->
```console
$ export TMH_CLIENT_ID=cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
$ export TMH_CLIENT_SECRET=tmh_cs_...
$ tmh whoami --live
Organization  org_acme
Scopes        learners:read, learners:write
Environment   Live
```

In GitHub Actions, map them from secrets:

<!-- doctest: illustrative -->
```yaml
env:
  TMH_CLIENT_ID: ${{ secrets.TMH_CLIENT_ID }}
  TMH_CLIENT_SECRET: ${{ secrets.TMH_CLIENT_SECRET }}
```

## Scopes

An Access token carries the Scopes its API client holds, such as
`learners:read` or `webhooks:manage`. `tmh whoami` lists yours, and each
command's `--help` and the [resource reference](../reference/resources.md#scopes)
name the Scopes it needs. A command whose Scopes your API client lacks fails
with exit code 2:

<!-- doctest: missing-scope -->
```console
$ tmh certificates list
Error: Forbidden: The access token lacks the certificates:read scope.
Request ID: req_01JD8S
$ echo $?
2
```

To change an API client's Scopes, edit it in the console; the next command
gets a token with the new set.

## The Organization binding

Most API paths start with `/organizations/{orgId}`. `tmh` asks the API which
Organization your credentials belong to (`tmh auth introspect` shows the
answer) and fills it in, so an Organization id is never an argument. That is
also why `tmh login` can say which Organization you logged in to.

## Public lookups without credentials

A few operations are public: the Course catalog (`tmh courses list`,
`tmh courses retrieve`) and Certificate verification
(`tmh certificate-verifications retrieve`). They send no Access token and
work before `tmh login`:

<!-- doctest: public-lookup -->
```console
$ tmh certificate-verifications retrieve 7Hk2pQ
{
  "status": "valid",
  "learnerName": "Ana Example",
  "courseTitle": "HIPAA Training 2026",
  "issuedAt": "2026-11-10T15:00:00.000Z",
  "expiresAt": "2027-11-10T15:00:00.000Z",
  "issuer": "TeachMeHIPAA"
}
```

The tiny id is the last part of a Certificate's `https://tme.so/<tinyId>`
link. [Raw requests](commands.md#raw-requests) to these paths are public
too, and `--no-auth` sends any other raw request without a token.
