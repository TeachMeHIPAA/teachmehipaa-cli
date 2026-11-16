# Exit codes

`tmh` exits 0 on success and with one code per kind of failure, so a script
tells a missing Learner from an outage without reading error text. Errors
print on stderr, never stdout.

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

Codes 6 and 7 come after `tmh` has retried the request, where a retry is
safe; see [idempotency keys and retries](requests.md#idempotency-keys-and-retries).

## Error output

An API error prints its title and detail, one line per field error, and the
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

Support finds the request in TeachMeHIPAA's logs by its request id, so
quote it. The API's problem also has a machine-readable `code`, such as
`validation_failed` or `seat_unavailable`. `tmh` maps the problem's HTTP
status to the exit code; scripts branch on that.

A usage mistake fails before anything is sent:

<!-- doctest: usage-error -->
```console
$ tmh learners retrieve
Error: accepts 1 arg(s), received 0
$ echo $?
1
```

An unreachable API fails after the retries:

<!-- doctest: connection-error -->
```console
$ tmh compliance retrieve
Error: could not reach the TeachMeHIPAA API: Get "https://sandbox.api.tme.so/v1/organizations/org_acme/compliance": dial tcp: lookup sandbox.api.tme.so: no such host
$ echo $?
7
```

## Branching on exit codes

`case $?` reads one call's outcome. Run the command without `set -e`, or it
exits the script before `case` runs:

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

`--verbose` shows each HTTP attempt, retries included, with its status and
request id; see [output](output.md#--verbose). More patterns are in
[recipes](recipes.md#gate-a-ci-job-on-the-compliance-rate).
