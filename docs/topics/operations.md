# Operations and files

<!-- doctest: synopsis -->
```text
tmh learners export --format csv [--status <status>] -o <file>
tmh enrollments export --format csv [--wait] [--wait-timeout <duration>]
tmh certificates export --format csv -o <file>
tmh certificates download-pdf <certId> -o <file>
tmh enrollments import --file <path> [--preview] [--wait]
tmh operations retrieve <operationId> [--wait]
```

Exports, imports, batch changes, a Webhook subscription's Test delivery and
a redrive take longer than one request. Their commands start an
**Operation** and print it straight away.

## --wait

An Operation has a `name`. It is `done` once it holds a `response` or an
`error`. `--wait` polls until then, for up to `--wait-timeout` (default 5m),
and prints the finished Operation:

<!-- doctest: wait -->
```console
$ tmh learners export --format csv --wait
{
  "name": "organizations/org_acme/operations/op_01JD84",
  "done": true,
  "metadata": {
    "kind": "learners.export"
  },
  "response": {
    "format": "csv",
    "contentType": "text/csv",
    "filename": "learners-2026-11-15.csv",
    "rowCount": 1,
    "dataBase64": "ZW1haWwsbmFtZQ0KYW5hQGV4YW1wbGUuY29tLEFuYSBFeGFtcGxlDQo="
  }
}
```

The first poll comes after 1 second; the gap then grows to at most 10
seconds. Without `--wait`, check on it later by id:

<!-- doctest: illustrative -->
```sh
tmh operations retrieve op_01JD84
```

A failed Operation prints with its problem, and `tmh` exits with the code
for the problem's HTTP `status`, as for any [API error](exit-codes.md). A
problem with no status exits 7. So does a `--wait-timeout` that runs out,
while the Operation carries on.

<!-- doctest: wait-failed -->
```console
$ tmh enrollments export --format csv --wait
{
  "name": "organizations/org_acme/operations/op_01JD84",
  "done": true,
  "metadata": {
    "kind": "enrollments.export"
  },
  "error": {
    "code": "export_too_large",
    "title": "Export too large",
    "status": 422,
    "detail": "Narrow the filter to under 10000 rows."
  }
}
Error: operation organizations/org_acme/operations/op_01JD84 failed: Export too large: Narrow the filter to under 10000 rows.
$ echo $?
5
```

## CSV exports

`learners export`, `enrollments export` and `certificates export` start an
export whose response carries the file. `-o <file>` implies `--wait` and
writes the CSV there; `-o -` writes it to stdout. The confirmation goes to
stderr:

<!-- doctest: export -->
```console
$ tmh learners export --format csv --status expired -o expired.csv
Saved learners-2026-11-15.csv (2 rows) to expired.csv
$ cat expired.csv
email,name,status
dee@example.com,Dee Example,expired
eli@example.com,Eli Example,expired
```

[export-certificates.sh](../../examples/export-certificates.sh) exports
every Certificate this way.

## Certificate PDFs

`tmh certificates download-pdf` writes a Certificate's PDF byte for byte. It
won't write a PDF to a terminal: give `-o <file>`, or `-o -` in a pipe.

<!-- doctest: download-pdf -->
```console
$ tmh certificates download-pdf cert_01JD85 -o certificate.pdf
Saved 18432 bytes to certificate.pdf
```

<!-- doctest: illustrative -->
```sh
tmh certificates download-pdf cert_01JD85 -o - | lpr
```

## CSV imports

`tmh enrollments import` invites everyone in a CSV with an `email` column
and optional `name` and `courseId` columns, up to 500 rows. `--preview`
checks the file and invites no one:

<!-- doctest: import-preview -->
```console
$ cat roster.csv
email,name
ana@example.com,Ana Example
not-an-email,Ben Example
$ tmh enrollments import --file roster.csv --preview
{
  "totalRows": 2,
  "validRows": 1,
  "seatsRequired": 1,
  "seatsAvailable": 12,
  "rows": [
    {
      "rowNumber": 1,
      "email": "ana@example.com",
      "name": "Ana Example",
      "courseId": "hipaa-2026",
      "status": "valid"
    },
    {
      "rowNumber": 2,
      "email": "not-an-email",
      "name": "Ben Example",
      "courseId": "hipaa-2026",
      "status": "invalid_email",
      "detail": "Not an email address."
    }
  ]
}
```

Drop `--preview` to import. `--file -` reads the CSV from stdin, and
`--options.course-id` picks the Course in place of the Organization's
default:

<!-- doctest: illustrative -->
```sh
tmh enrollments import --file roster.csv --wait
generate-roster | tmh enrollments import --file - --options.course-id hipaa-2026 --wait
```

The import sends a generated Idempotency-Key, and a retry resends the same
bytes under the same key. A dropped connection can't import the file twice.

## Batch changes

`tmh learners batch-upsert`, `tmh enrollments batch` and
`tmh enrollments batch-by-filter` change many records in one request. Their
bodies hold lists, so pass them with `-d key:=<json>`, `--data @file.json`
or `--data @-`:

<!-- doctest: illustrative -->
```sh
tmh enrollments batch --operation remind \
  --select.enrollment-ids enr_01JD8K2M4N6P8Q0R2S4T6V8W0X,enr_01JD8K2M4N6P8Q0R2S4T6V8W0Y
jq -n '{items: [{externalId: "hr-1042", email: "dana.whitfield@example.com"}]}' |
  tmh learners batch-upsert --data @- --wait
```

`enrollments batch` answers at once with a result per Enrollment;
`learners batch-upsert` and `enrollments batch-by-filter` return an
Operation. The [resource reference](../reference/resources.md) lists each
command's fields.
