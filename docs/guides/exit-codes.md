# Exit codes and errors

`tmh` exits 0 on success and with a distinct code for each kind of failure,
so a script can tell a missing Learner from an outage without parsing error
text. Errors print on stderr, never stdout.

## Exit codes

| Exit code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Usage error or local failure: a bad flag, argument or input file, or anything that went wrong on this machine rather than at the API, such as an unreadable config file. |
| `2` | Auth failed: not logged in, `401`, `403`, or the token endpoint refused the credentials. |
| `3` | Not found: `404`, or something `tmh` looked for isn't there, such as an Event for `events resend`. |
| `4` | Conflict or precondition failed: `409`, `412` or `428`. |
| `5` | Invalid request: any other `4xx`, such as `400` or `422`. |
| `6` | Rate limited: `429`, after retries. |
| `7` | Server or connection error: `5xx`, no response, a response that isn't the API's, or under `--wait` a timed-out Operation or a failed one whose problem has no status. |
| `130` | Interrupted with Ctrl-C. |

Codes 6 and 7 come after `tmh` has already retried the request where that is
safe; see [idempotency keys and retries](commands.md#idempotency-keys-and-retries).

## How errors print

An API error prints its title and detail, each field error under it, and the
request id:

<!-- doctest: validation-error -->
```console
$ tmh learners create --email ana@example
Error: Validation failed: The request has invalid fields.
  email: Invalid email
Request ID: req_01JD8Q
$ echo $?
5
```

Quote the request id when you contact support; it identifies the request in
TeachMeHIPAA's logs. The API's problems also carry a machine-readable `code`,
such as `validation_failed` or `seat_unavailable`, which `tmh` maps to the
exit code through the HTTP status; branch on the exit code in scripts.

A usage mistake is caught before anything is sent:

<!-- doctest: usage-error -->
```console
$ tmh learners retrieve
Error: accepts 1 arg(s), received 0
$ echo $?
1
```

When the API can't be reached at all, `tmh` says so after its retries:

<!-- doctest: connection-error -->
```console
$ tmh compliance retrieve
Error: could not reach the TeachMeHIPAA API: Get "https://api-teachmehipaa-dev.web.app/v1/organizations/org_acme/compliance": dial tcp: lookup api-teachmehipaa-dev.web.app: no such host
$ echo $?
7
```

## Branching on exit codes

<!-- doctest: illustrative -->
```sh
#!/bin/sh
tmh learners upsert-by-external-id "$external_id" --email "$email" > /dev/null
case $? in
  0) ;;
  4) echo "conflict for $external_id; check the External ID" >&2 ;;
  5) echo "invalid row for $external_id" >&2 ;;
  6 | 7) echo "TeachMeHIPAA unavailable; try again later" >&2; exit 1 ;;
  *) exit 1 ;;
esac
```

`--verbose` shows each HTTP attempt, including the retries, with its status
and request id; see [Output and scripting](output-and-scripting.md#seeing-the-http-requests).
