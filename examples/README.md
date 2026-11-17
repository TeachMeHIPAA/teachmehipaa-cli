# Examples

Bash scripts that run `tmh` against the Sandbox Environment. Each reads a
Sandbox API client's credentials from `TMH_CLIENT_ID` and
`TMH_CLIENT_SECRET`, needs no `tmh login`, and exits with a message when
either is unset. Where noted, they need [jq](https://jqlang.org/).

| Example | What it does |
|---|---|
| [`quickstart.sh`](quickstart.sh) | Prints the Compliance summary and the first five Learners |
| [`sync-roster.sh`](sync-roster.sh) | Syncs a roster CSV by External ID with `learners upsert-by-external-id`, then lists Learners no longer on it |
| [`export-certificates.sh`](export-certificates.sh) | Exports every Certificate to CSV with `--wait -o`, and optionally downloads their PDFs |
| [`webhook-dev-loop.sh`](webhook-dev-loop.sh) | Forwards Events to your local webhook handler with `events tail --forward-to`, optionally starting the handler |

## Running them

[Install](../README.md#install) `tmh`, then:

<!-- doctest: illustrative -->
```sh
export TMH_CLIENT_ID=cli_...
export TMH_CLIENT_SECRET=tmh_cs_...
examples/quickstart.sh
examples/sync-roster.sh roster.csv
examples/export-certificates.sh certificates.csv --pdfs
examples/webhook-dev-loop.sh http://localhost:3000/webhooks/tmh -- node server.js
```

The scripts never pass `--live`, so every `tmh` command in them uses the
`sandbox` profile. CI lints them with [ShellCheck](https://www.shellcheck.net/),
and the test suite checks each `tmh` command, flag and `jq` program in them.
Nothing in CI runs them end to end.
