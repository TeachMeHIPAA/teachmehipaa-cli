# Commands

Every Developer API operation has a command named `tmh <resource> <method>`,
generated from the same description of the API as the SDKs. This guide
explains how a command's arguments and flags become a request. The
[command reference](../reference/commands/tmh.md) lists every command and
flag, and the [resource reference](../reference/resources.md) maps each one
to its HTTP route.

## Resources and methods

The resources are `audit-events`, `auth`, `certificate-verifications`,
`certificates`, `compliance`, `courses`, `enrollments`, `events`,
`learners`, `members`, `operations`, `seats` and `webhook-subscriptions`
(also spelled `webhooks`). Methods are the API's, in kebab-case:

<!-- doctest: illustrative -->
```sh
tmh learners list --status in_progress
tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M
tmh learners upsert-by-external-id hr-123 --email ana@example.com
tmh enrollments cancel enr_01JD81CD2E3F4G5H6J7K8M9N0P --refund
tmh webhook-subscriptions test whs_01JD82EF3G4H5J6K7M8N9P0Q1R
```

`tmh <resource> --help` lists a resource's methods, and
`tmh <resource> <method> --help` shows the API's description of the
operation, the Scopes it needs and its flags.

## Arguments and flags

- **Path ids are positional arguments**, in the order the path has them:
  `tmh learners retrieve <learnerId>`. The Organization id is never one:
  `tmh` fills it in from your credentials.
- **Query parameters and body fields are flags.** Names are kebab-case, and
  nested body fields are dotted: `--course-id`, `--options.course-id`. Enum
  flags list their values in `--help`, and list flags take comma-separated
  values (`--event-types learner.certified,learner.expired`).
- **Required flags** are marked `(required)` in `--help`. A required query
  flag must be given; a required body field may also come from `-d` or
  `--data`.

## Building a request body

Besides the flags, three ways set body fields, so you can send anything the
API accepts, including fields newer than your `tmh`:

- **`-d key=value`** sets one field. Dotted keys nest
  (`-d options.courseId=hipaa-2026`), `key[]=v` appends to an array, and
  `key:=<json>` sends a JSON value instead of a string (`-d externalId:=null`,
  `-d seatsLowThreshold:=3`). Keys use the API's camelCase names.
- **`--data @file.json`** sends a whole JSON document as the body, `--data @-`
  reads it from stdin, and `--data '{"email":"ana@example.com"}'` takes it
  inline.
- Flags override `-d` fields, which override the `--data` document.

<!-- doctest: illustrative -->
```sh
tmh webhook-subscriptions create \
  --endpoint-url https://example.com/webhooks/tmh \
  --event-types learner.certified,learner.expired \
  -d seatsLowThreshold:=3
tmh webhook-subscriptions create --data @subscription.json
jq -n '{email: "ana@example.com", name: "Ana Example"}' | tmh learners create --data @-
```

## Updates, ETags and `--if-match`

Updates are merge patches: fields you leave out keep their current values,
and `-d field:=null` clears a nullable field. An update that needs a
precondition takes `--if-match <etag>`, the `ETag` header of the resource as
you last read it. `--show-headers` prints it:

<!-- doctest: illustrative -->
```console
$ tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M --show-headers
…
< Etag: "v7"
…
```

<!-- doctest: update-if-match -->
```console
$ tmh learners update sub_01JD80AB2C3D4E5F6G7H8J9K0M --name "Ana B. Example" --if-match '"v7"'
{
  "learnerId": "sub_01JD80AB2C3D4E5F6G7H8J9K0M",
  "email": "ana@example.com",
  "name": "Ana B. Example",
  "externalId": "hr-123",
  "status": "in_progress",
  "course": {
    "courseId": "hipaa-2026",
    "title": "HIPAA Training 2026"
  },
  "invitedAt": "2026-11-01T09:00:00.000Z",
  "startedAt": "2026-11-02T10:30:00.000Z",
  "completedAt": null,
  "dueAt": "2026-12-01T00:00:00.000Z",
  "certificate": null
}
```

If someone changed the Learner since you read it, the ETag no longer matches
and the update fails with exit code 4 instead of overwriting their change.
Read it again and retry:

<!-- doctest: update-precondition-failed -->
```console
$ tmh learners update sub_01JD80AB2C3D4E5F6G7H8J9K0M --name "Ana B. Example" --if-match '"v6"'
Error: Precondition failed: The learner changed since you read it. Retrieve it again for its current ETag.
Request ID: req_01JD8T
$ echo $?
4
```

## Idempotency keys and retries

Writes that create something, such as `learners create` or
`enrollments import`, carry an `Idempotency-Key` so that a replay never
happens twice. `tmh` generates one for each command and reuses it for every
retry of that command. Pass `--idempotency-key <key>` to choose your own, for
example to make a whole script safe to re-run:

<!-- doctest: illustrative -->
```sh
tmh learners create --email ana@example.com --idempotency-key "onboard-hr-123"
```

`tmh` retries transient failures (connection errors, `429`, `5xx` and a
`409 conflict`) of requests that are safe to replay twice, with backoff,
honoring `Retry-After`, as the SDKs do. `--max-retries <n>` changes the
count and `--max-retries 0` turns retries off. The
[resource reference](../reference/resources.md) says which commands are
retried; updates (`PATCH`) and deletes never are.

## Raw requests

`tmh get|post|patch|put|delete <path>` calls any API path with the same
authentication, retries and error handling as the generated commands, for
anything they don't cover yet. The path is relative to the base URL, which
already ends in `/v1`, and a literal `{orgId}` is replaced by your
Organization:

<!-- doctest: raw-get -->
```console
$ tmh get /organizations/{orgId}/learners --query limit=1 --query status=compliant
{
  "items": [
    {
      "learnerId": "sub_01JD80AB2C3D4E5F6G7H8J9K0N",
      "email": "cam@example.com",
      "name": "Cam Example",
      "externalId": "hr-125",
      "status": "compliant",
      "course": {
        "courseId": "hipaa-2026",
        "title": "HIPAA Training 2026"
      },
      "invitedAt": "2026-10-01T09:00:00.000Z",
      "startedAt": "2026-10-02T10:30:00.000Z",
      "completedAt": "2026-10-03T16:45:00.000Z",
      "dueAt": "2026-11-01T00:00:00.000Z",
      "certificate": {
        "certificateId": "cert_01JD85",
        "issuedAt": "2026-10-03T16:45:00.000Z",
        "expiresAt": "2027-10-03T16:45:00.000Z",
        "verifyUrl": "https://tme.so/7Hk2pQ",
        "pdfUrl": "https://api.tme.so/v1/organizations/org_acme/certificates/cert_01JD85/content"
      }
    }
  ],
  "nextCursor": null
}
```

<!-- doctest: illustrative -->
```sh
tmh post /organizations/{orgId}/learners -d email=ana@example.com -d name="Ana Example"
tmh patch /organizations/{orgId}/learners/sub_01JD80AB2C3D4E5F6G7H8J9K0M -d externalId:=null --if-match '"v7"'
```

`-d`, `--data`, `--if-match` and `--idempotency-key` work as on the generated
commands, `--query key=value` adds a query parameter, and `--content-type`
overrides the body's type. PATCH bodies default to
`application/merge-patch+json`. `tmh get -o <file>` saves the response body
unchanged, whatever its type.

A path the API serves publicly (`/courses`, `/courses/<courseId>` and
`/certificate-verifications/<tinyId>`) is sent without an Access token, so it
works before `tmh login`. `--no-auth` sends any other request without one
too.

A raw GET is retried like any read. A raw POST is retried with the
Idempotency-Key `tmh` generates for it, and a raw PUT only when you pass
`--idempotency-key`. A raw PATCH or DELETE is never retried, key or not,
since it may already have taken effect.

## Opening pages

`tmh open docs` opens the Developer API documentation and `tmh open console`
the TeachMeHIPAA console, where admins manage API clients. Without a
browser, `tmh open` prints the URL.
