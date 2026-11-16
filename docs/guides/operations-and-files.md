# Operations and files

Some work takes longer than one request: exports, imports, batch changes, a
Webhook subscription's Test delivery or redrive. Those commands start an
**Operation** and print it at once. This guide covers waiting for
Operations, and the commands that move files: CSV exports, Certificate PDFs
and CSV imports.

## Waiting with `--wait`

An Operation has a `name` and is `done` once it has a `response` or an
`error`. `--wait` polls it until it is done, up to `--wait-timeout` (5
minutes by default), and prints the finished Operation:

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

`tmh` waits 1 second, then backs off to at most 10 seconds between polls.
Without `--wait`, print the Operation and check it later by name:

<!-- doctest: illustrative -->
```sh
tmh operations retrieve op_01JD84
```

If the Operation failed, `tmh` prints it and its problem, and exits with the
code the problem's HTTP `status` maps to, as for any
[API error](exit-codes.md), or 7 when the problem has no status. A
`--wait-timeout` that runs out exits 7 too; the Operation keeps running.

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

## Exporting CSV

`tmh learners export`, `tmh enrollments export` and
`tmh certificates export` start an export Operation whose response holds the
file. `-o <file>` implies `--wait` and saves the finished CSV; `-o -` writes
it to stdout. The confirmation goes to stderr:

<!-- doctest: export -->
```console
$ tmh learners export --format csv --status expired -o expired.csv
Saved learners-2026-11-15.csv (2 rows) to expired.csv
$ cat expired.csv
email,name,status
dee@example.com,Dee Example,expired
eli@example.com,Eli Example,expired
```

The [Certificate export example](../../examples/export-certificates.sh)
exports every Certificate this way.

## Downloading Certificate PDFs

`tmh certificates download-pdf` saves a Certificate's PDF byte for byte. It
refuses to print a PDF to a terminal; use `-o <file>`, or `-o -` to pipe it:

<!-- doctest: download-pdf -->
```console
$ tmh certificates download-pdf cert_01JD85 -o certificate.pdf
Saved 18432 bytes to certificate.pdf
```

<!-- doctest: illustrative -->
```sh
tmh certificates download-pdf cert_01JD85 -o - | lpr
```

## Importing a CSV roster

`tmh enrollments import` invites everyone in a CSV file with an `email`
column, and optional `name` and `courseId` columns, up to 500 rows. Check the
file first with `--preview`, which reports what would happen without
inviting anyone:

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

Then import it for real and wait for the result. `--file -` reads the CSV
from stdin, and `--options.course-id` sets the Course to invite everyone to
instead of the Organization's default:

<!-- doctest: illustrative -->
```sh
tmh enrollments import --file roster.csv --wait
generate-roster | tmh enrollments import --file - --options.course-id hipaa-2026 --wait
```

An import carries a generated Idempotency-Key, and a retry resends the same
bytes with the same key, so a network failure never imports the file twice.

## Batch changes

`tmh learners batch-upsert`, `tmh enrollments batch` and
`tmh enrollments batch-by-filter` change many records in one Operation. Their
bodies are lists, so send them with `--data @file.json` or `--data @-`; the
[resource reference](../reference/resources.md) lists each one's fields.
