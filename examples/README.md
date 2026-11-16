# Examples

Shell scripts that use `tmh` against the Sandbox Environment. Each reads your
Sandbox API client's credentials from `TMH_CLIENT_ID` and
`TMH_CLIENT_SECRET`, so it needs no `tmh login`, and stops with a message if
they're unset. They need bash and, where noted, [jq](https://jqlang.org/).

| Example | What it does |
|---|---|
| [`quickstart.sh`](quickstart.sh) | Prints the Compliance summary and the first five Learners |
| [`sync-roster.sh`](sync-roster.sh) | Syncs a roster CSV by External ID with `learners upsert-by-external-id`, then lists Learners no longer on it |
| [`export-certificates.sh`](export-certificates.sh) | Exports every Certificate to CSV with `--wait -o`, and optionally downloads their PDFs |
| [`webhook-dev-loop.sh`](webhook-dev-loop.sh) | Forwards Events to your local webhook handler with `events tail --forward-to`, optionally starting the handler |

## Running them

Install `tmh` (see [Getting started](../docs/guides/getting-started.md)),
then:

<!-- doctest: illustrative -->
```sh
export TMH_CLIENT_ID=cli_...
export TMH_CLIENT_SECRET=tmh_cs_...
examples/quickstart.sh
examples/sync-roster.sh roster.csv
examples/export-certificates.sh certificates.csv --pdfs
examples/webhook-dev-loop.sh http://localhost:3000/webhooks/tmh -- node server.js
```

The scripts never touch Live: without `--live`, every `tmh` command uses the
`sandbox` profile. CI checks them with [ShellCheck](https://www.shellcheck.net/),
and the test suite checks that every `tmh` command and flag in them exists,
but nothing runs them.
