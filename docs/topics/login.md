# Login, profiles and Scopes

<!-- doctest: synopsis -->
```text
tmh login [--client-id <id>] [--live]
tmh logout [--live]
tmh whoami [--live]
tmh auth introspect [--live]
```

`tmh` signs in as an **API client**: a client id (`cli_…`) and a client
secret (`tmh_cs_…`) that an Organization admin creates in the console. An
API client belongs to one Organization and one Environment and holds a set
of Scopes. Each command trades it for an Access token, keeps the token in
memory until it exits, and never writes the token to disk.

## tmh login

<!-- doctest: illustrative -->
```console
$ tmh login
Client ID: cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
Client secret:
Logged in to Sandbox as Organization org_acme.
```

`tmh login` prompts for both values, checks them with TeachMeHIPAA, and only
then stores them. The secret goes to the OS keyring: the macOS Keychain, the
Windows Credential Manager or the Secret Service on Linux. Where no keyring
works, as in most CI containers, it goes to `config.toml` with mode 0600,
and `tmh` says so on stderr. The secret is not echoed as you type.

Without a terminal, pass the id as a flag and pipe the secret on stdin. It
stays out of your shell history and the process list:

<!-- doctest: login-piped -->
```console
$ printf %s "$TMH_SECRET" | tmh login --client-id cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
Logged in to Sandbox as Organization org_acme.
```

## tmh logout

<!-- doctest: logout -->
```console
$ tmh logout
Logged out of the sandbox profile.
```

`tmh logout` deletes the profile's stored client secret. The profile's local
Signing secret for [`events tail`](events.md#the-local-signing-secret)
stays.

## Profiles

| Profile | Environment | Base URL |
|---|---|---|
| `sandbox` (default) | Sandbox | `https://api-teachmehipaa-dev.web.app/v1` |
| `live` | Live | `https://api.tme.so/v1` |

Credentials belong to one Environment, so each profile has its own login.
A command reaches Live only with `--live` or `--profile live`:

<!-- doctest: illustrative -->
```sh
tmh login --live
tmh learners list --live
tmh whoami --profile live
```

A command for a profile with no login exits 2 and names the fix:

<!-- doctest: not-logged-in -->
```console
$ tmh compliance retrieve --live
Error: not logged in to the live profile; run `tmh login --live` or set TMH_CLIENT_ID and TMH_CLIENT_SECRET
$ echo $?
2
```

## Credentials from the environment

`TMH_CLIENT_ID` and `TMH_CLIENT_SECRET` override stored credentials, so a CI
job needs no `tmh login` and no keyring. Set both or neither. The profile
still picks the Environment, so Live credentials need `--live`:

<!-- doctest: env-credentials -->
```console
$ export TMH_CLIENT_ID=cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
$ export TMH_CLIENT_SECRET=tmh_cs_...
$ tmh whoami --live
Organization  org_acme
Scopes        learners:read, learners:write
Environment   Live
```

GitHub Actions maps them from repository secrets:

<!-- doctest: illustrative -->
```yaml
env:
  TMH_CLIENT_ID: ${{ secrets.TMH_CLIENT_ID }}
  TMH_CLIENT_SECRET: ${{ secrets.TMH_CLIENT_SECRET }}
```

## Scopes

`tmh whoami` lists the Scopes your API client holds, such as `learners:read`
or `webhooks:manage`. Each command's `--help` and the
[resource reference](../reference/resources.md#scopes) name the Scopes it
needs. Without them the API answers `403` and `tmh` exits 2:

<!-- doctest: missing-scope -->
```console
$ tmh certificates list
Error: Forbidden: The access token lacks the certificates:read scope.
Request ID: req_01JD8S
$ echo $?
2
```

An admin changes an API client's Scopes in the console. The next command
fetches a token with the new set.

## The Organization binding

Most API paths start with `/organizations/{orgId}`. `tmh` asks the API which
Organization the credentials belong to and fills in the id, so it is never
an argument. `tmh auth introspect` prints the full answer: the client id,
the Organization, the Scopes, the mode and the token's expiry.

## Public operations

Three operations need no credentials: `tmh courses list`,
`tmh courses retrieve` and `tmh certificate-verifications retrieve`. They
send no Access token and work before `tmh login`:

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

The tiny id is the last path segment of a Certificate's
`https://tme.so/<tinyId>` link. [Raw requests](requests.md#raw-requests) to
these paths skip the token too, and `--no-auth` sends any other raw request
without one.
