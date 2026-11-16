# Output

<!-- doctest: synopsis -->
```text
tmh learners list [--limit <n>] [--cursor <nextCursor>]
tmh learners list --all [--ndjson]
tmh events list [--after <eventId>] [--all [--ndjson]]
tmh learners retrieve <learnerId> --verbose
tmh learners retrieve <learnerId> --show-headers
```

stdout carries the response, as JSON. stderr carries errors, notices,
`--verbose` and `--show-headers`. A pipe into `jq` or a redirect to a file
gets the response and nothing else.

## JSON

On a terminal, `tmh` pretty-prints JSON in color; `NO_COLOR` turns the color
off. Piped or redirected, it prints compact JSON, one document per line:

<!-- doctest: compact-output -->
```console
$ tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M > ana.json
$ cat ana.json
{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K0M","email":"ana@example.com","name":"Ana Example","externalId":"hr-123","status":"in_progress","course":{"courseId":"hipaa-2026","title":"HIPAA Training 2026"},"invitedAt":"2026-11-01T09:00:00.000Z","startedAt":"2026-11-02T10:30:00.000Z","completedAt":null,"dueAt":"2026-12-01T00:00:00.000Z","certificate":null}
```

Fields keep the API's order. A field this `tmh` doesn't know, from a newer
API release, prints too.

<!-- doctest: illustrative -->
```sh
tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M | jq -r .status
tmh compliance retrieve | jq '.totals | {compliant, expired}'
```

## Pages

A list command prints one page: an object with `items` and `nextCursor`.
`--limit` sets the page size, and `--cursor` takes the previous page's
`nextCursor`. On `tmh events list`, `--cursor` is an alias for the event
log's `--after`.

<!-- doctest: illustrative -->
```sh
tmh learners list --limit 100 --cursor eyJvZmZzZXQiOjEwMH0
```

`--all` follows every page with the same flags and prints every item in one
JSON array:

<!-- doctest: list-all -->
```console
$ tmh learners list --status expired --all
[
  {
    "learnerId": "sub_01JD80AB2C3D4E5F6G7H8J9K1A",
    "email": "dee@example.com",
    "status": "expired"
  },
  {
    "learnerId": "sub_01JD80AB2C3D4E5F6G7H8J9K1B",
    "email": "eli@example.com",
    "status": "expired"
  },
  {
    "learnerId": "sub_01JD80AB2C3D4E5F6G7H8J9K1C",
    "email": "fay@example.com",
    "status": "expired"
  }
]
```

Those three Learners arrived on two pages. `--ndjson` prints one compact
item per line as each page lands, which suits `while read` loops, `grep` and
large lists:

<!-- doctest: list-ndjson -->
```console
$ tmh learners list --status expired --all --ndjson
{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K1A","email":"dee@example.com","status":"expired"}
{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K1B","email":"eli@example.com","status":"expired"}
{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K1C","email":"fay@example.com","status":"expired"}
```

<!-- doctest: illustrative -->
```sh
tmh learners list --all | jq length
tmh audit-events list --all --ndjson > audit.ndjson
tmh learners list --status expired --all --ndjson | jq -r .email
```

Ctrl-C during `--all` leaves the array unclosed and exits 130, so a script
can't mistake part of a list for the whole.

## --verbose

`--verbose` prints one line per HTTP attempt on stderr: method, path,
status, latency, request id and, for writes, the Idempotency-Key. A retried
request prints a line per attempt.

<!-- doctest: verbose -->
```console
$ tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M --verbose > /dev/null
POST /v1/auth/token 200 84ms request-id=req_01JD8P
GET /v1/auth/introspection 200 35ms request-id=req_01JD8Q
GET /v1/organizations/org_acme/learners/sub_01JD80AB2C3D4E5F6G7H8J9K0M 200 41ms request-id=req_01JD8R
```

## --show-headers

`--show-headers` prints the `--verbose` line for each attempt, then that
response's headers on stderr, one `< Name: value` line each, `ETag` and
`Retry-After` among them.
Neither flag prints a request or response body or a credential:
`Authorization` and cookie headers show as `[redacted]`.

Exit codes for scripts are in [exit-codes](exit-codes.md); worked shell
examples are in [recipes](recipes.md).
