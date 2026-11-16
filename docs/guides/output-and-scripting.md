# Output and scripting

`tmh` is built to be piped. Responses go to stdout as JSON; errors, notices
and `--verbose` lines go to stderr, so they never corrupt what you pipe. Each
kind of failure exits with its own [exit code](exit-codes.md).

## JSON output

On a terminal, `tmh` pretty-prints JSON in color. When stdout is piped or
redirected, it prints compact JSON, one document per line, so `tmh … | jq`
just works. Set `NO_COLOR` to keep the terminal output uncolored.

<!-- doctest: compact-output -->
```console
$ tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M > ana.json
$ cat ana.json
{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K0M","email":"ana@example.com","name":"Ana Example","externalId":"hr-123","status":"in_progress","course":{"courseId":"hipaa-2026","title":"HIPAA Training 2026"},"invitedAt":"2026-11-01T09:00:00.000Z","startedAt":"2026-11-02T10:30:00.000Z","completedAt":null,"dueAt":"2026-12-01T00:00:00.000Z","certificate":null}
```

Fields are printed as the API sent them, in its order. A field `tmh` doesn't
know yet, from a newer API version, is printed too.

<!-- doctest: illustrative -->
```sh
tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M | jq -r .status
tmh compliance retrieve | jq '.totals | {compliant, expired}'
```

## Pages, `--all` and `--ndjson`

List commands print one page: an object with `items` and `nextCursor`.
`--limit` sets the page size and `--cursor` takes the `nextCursor` of the
previous page. On `tmh events list`, `--cursor` is another name for the event
log's `--after`.

<!-- doctest: illustrative -->
```sh
tmh learners list --limit 100 --cursor eyJvZmZzZXQiOjEwMH0
```

`--all` follows every page, keeping the other flags, and prints all the items
as one JSON array:

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

Here the three Learners came from two pages; `--limit` sets how many each
page holds. Add `--ndjson` to print one compact item per line as each page
arrives instead, which suits large exports and line-oriented tools:

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

If `--all` is interrupted, the array is left unfinished and `tmh` exits 130,
so a script never mistakes part of a list for all of it.

## Scripting with exit codes

Branch on the exit code rather than parsing error text. `tmh` exits 0 only on
success; the [exit codes](exit-codes.md) tell a missing Learner (3) from a
conflict (4) or an outage (7):

<!-- doctest: illustrative -->
```sh
#!/bin/sh
set -eu
if tmh learners retrieve "$learner_id" > learner.json; then
  jq -r .status learner.json
else
  status=$?
  [ "$status" -eq 3 ] && echo "no such Learner" >&2
  exit "$status"
fi
```

Use `set -o pipefail` in bash when you pipe `tmh` into `jq`, so a failed
`tmh` fails the pipeline.

## Seeing the HTTP requests

`--verbose` prints one line per HTTP attempt to stderr: method, path, status,
latency, request id and, for writes, the Idempotency-Key. A retried request
shows each attempt.

<!-- doctest: verbose -->
```console
$ tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M --verbose > /dev/null
POST /v1/auth/token 200 84ms request-id=req_01JD8P
GET /v1/auth/introspection 200 35ms request-id=req_01JD8Q
GET /v1/organizations/org_acme/learners/sub_01JD80AB2C3D4E5F6G7H8J9K0M 200 41ms request-id=req_01JD8R
```

`--show-headers` adds each response's headers, such as `ETag` and
`Retry-After`. Neither ever prints a request or response body or a
credential; `Authorization` and cookie headers are redacted.
