# Requests

<!-- doctest: synopsis -->
```text
tmh learners retrieve <learnerId>
tmh learners create --email <email> [--name <name>] [-d <key>=<value>]
tmh learners update <learnerId> --if-match <etag> [-d <key>:=<json>]
tmh webhook-subscriptions create --data @<file>
tmh get <path> [--query <key>=<value>] [--no-auth] [-o <file>]
tmh post <path> [-d <key>=<value>] [--content-type <type>]
tmh patch <path> --if-match <etag> [-d <key>:=<json>]
tmh open docs
```

Each Developer API operation is a generated command, `tmh <resource>
<method>`, built from the same API description as the SDKs. This page shows
how its arguments and flags become an HTTP request. The
[resource reference](../reference/resources.md) maps every command to its
route.

## Resources and methods

The resources are `audit-events`, `auth`, `certificate-verifications`,
`certificates`, `compliance`, `courses`, `enrollments`, `events`,
`learners`, `members`, `operations`, `seats` and `webhook-subscriptions`
(alias `webhooks`). Methods keep the API's names in kebab-case:

<!-- doctest: illustrative -->
```sh
tmh learners list --status in_progress
tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M
tmh learners upsert-by-external-id hr-1042 --email dana.whitfield@example.com
tmh enrollments cancel enr_01JD8K2M4N6P8Q0R2S4T6V8W0X --refund
tmh webhook-subscriptions test whs_01JD8Q5R7S9T1V3W5X7Y9Z1A3B
```

`tmh <resource> --help` lists a resource's methods. `tmh <resource> <method>
--help` prints the API's description of the operation, its Scopes and its
flags.

## Arguments and flags

- Path ids are positional, in path order: `tmh learners retrieve
  <learnerId>`. The Organization id never is; `tmh` fills it in.
- Query parameters and body fields are flags, in kebab-case. Nested body
  fields use dots: `--options.course-id`. Enum flags list their values in
  `--help`. List flags take commas: `--event-types
  learner.certified,learner.expired`.
- `--help` marks required flags `(required)`. A required query flag must
  appear on the command line. A required body field can also come from `-d`
  or `--data`.

Body fields as flags:

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

## -d and --data

Two more ways set body fields. Both reach fields that have no flag, or that
are newer than your `tmh`:

- `-d key=value` sets one field as a string. Dotted keys nest
  (`-d options.courseId=hipaa-2026`), `key[]=v` appends to an array, and
  `key:=<json>` sends a JSON value (`-d externalId:=null`,
  `-d seatsLowThreshold:=3`). Keys use the API's camelCase names.
- `--data @file.json` sends a whole JSON document as the body, `--data @-`
  reads one from stdin, and `--data '{"email":"dana.whitfield@example.com"}'`
  takes it inline.

Flags beat `-d` fields, and `-d` fields beat the `--data` document.

<!-- doctest: illustrative -->
```sh
tmh webhook-subscriptions create \
  --endpoint-url https://example.com/webhooks/teachmehipaa \
  --event-types learner.certified,learner.expired \
  -d seatsLowThreshold:=3
tmh webhook-subscriptions create --data @subscription.json
jq -n '{email: "dana.whitfield@example.com", name: "Dana Whitfield"}' | tmh learners create --data @-
```

## ETags and --if-match

Updates are merge patches. Fields you leave out keep their values, and
`-d field:=null` clears a nullable one. `tmh learners update` and
`tmh webhook-subscriptions update` refuse to run without `--if-match <etag>`:
the `ETag` the resource had when you read it. `--show-headers` prints it on
stderr:

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

If someone changed the Learner after your read, the ETag no longer matches.
The API answers `412`, `tmh` exits 4, and their change survives. Read the
Learner again and retry:

<!-- doctest: update-precondition-failed -->
```console
$ tmh learners update sub_01JD80AB2C3D4E5F6G7H8J9K0M --name "Ana B. Example" --if-match '"v6"'
Error: Precondition failed: The learner changed since you read it. Retrieve it again for its current ETag.
Request ID: req_01JD8T
$ echo $?
4
```

In a script, capture the ETag from stderr and pass it on.
[Recipes](recipes.md#update-a-learner-with-its-current-etag) has the
two-line version.

## Idempotency keys and retries

Writes that create something, such as `learners create` or
`enrollments import`, send an `Idempotency-Key`. `tmh` generates one per
command and reuses it on every retry, so a retried request never takes
effect twice. `--idempotency-key <key>` sets your own, which makes a whole
script safe to re-run:

<!-- doctest: illustrative -->
```sh
tmh learners create --email dana.whitfield@example.com --idempotency-key onboard-hr-1042
```

`tmh` retries connection errors, `429`, `5xx` and `409 conflict`, with
backoff and `Retry-After`, on requests that are safe to replay.
`--max-retries <n>` changes the count from 2; `--max-retries 0` turns
retries off. `PATCH` and `DELETE` are never retried. The
[resource reference](../reference/resources.md) has a Retries row for each
command.

## Raw requests

`tmh get`, `post`, `put`, `patch` and `delete` call any API path with the
same authentication, retries and error output as the generated commands.
The path is relative to the base URL, which already ends in `/v1`. A literal
`{orgId}` becomes your Organization:

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
tmh post /organizations/{orgId}/learners -d email=dana.whitfield@example.com -d name="Dana Whitfield"
tmh patch /organizations/{orgId}/learners/sub_01JD80AB2C3D4E5F6G7H8J9K0M -d externalId:=null --if-match '"v7"'
```

| Flag | Raw requests |
|---|---|
| `-d`, `--data`, `--if-match`, `--idempotency-key` | Work as on the generated commands. |
| `--query key=value` | Adds a query parameter; repeatable. |
| `--content-type <type>` | Overrides the body's type. `PATCH` defaults to `application/merge-patch+json`, the others to `application/json`. |
| `--no-auth` | Sends no Access token. `/courses`, `/courses/<courseId>` and `/certificate-verifications/<tinyId>` never get one. |
| `-o <file>` | On `tmh get`, saves the response body byte for byte, whatever its type. |

A raw `GET` retries like any read. A raw `POST` retries with the
Idempotency-Key `tmh` generates for it. A raw `PUT` retries only with
`--idempotency-key`. A raw `PATCH` or `DELETE` never retries, key or not,
since the first attempt may have taken effect.

## tmh open

`tmh open docs` (or `reference`) opens the Developer API documentation.
`tmh open console` (or `dashboard`) opens the TeachMeHIPAA console, where
admins manage API clients. With no browser available, `tmh open` prints the
URL.
