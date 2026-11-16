# Recipes

Shell workflows built from `tmh`, `jq` and POSIX tools. Each one assumes a
`tmh login` on the `sandbox` profile; add `--live` to the `tmh` commands to
run it against Live. The test suite checks every `tmh` command and flag on
this page against the real command tree and compiles every `jq` program.
The pipelines need [jq](https://jqlang.org/), and `set -o pipefail` needs
bash, zsh or another shell that has it.

## Count Learners by status

<!-- doctest: illustrative -->
```sh
tmh learners list --all --ndjson | jq -r .status | sort | uniq -c
```

`--all --ndjson` prints every Learner on every page, one per line, which is
the shape `sort` and `uniq` read.

## Write expired Learners to a CSV file

<!-- doctest: illustrative -->
```sh
set -o pipefail
{
  echo email,name,dueAt
  tmh learners list --status expired --all --ndjson |
    jq -r '[.email, .name, .dueAt] | @csv'
} > expired.csv
```

`@csv` quotes each field, so a comma in a name stays inside its column. To
let the API build the file, with its own columns, ask for an export:

<!-- doctest: illustrative -->
```sh
tmh learners export --format csv --status expired -o expired.csv
```

## Remind everyone who hasn't started

<!-- doctest: illustrative -->
```sh
tmh learners list --status not_started --all --ndjson |
  jq -r .learnerId |
  while read -r id; do
    tmh learners remind "$id" > /dev/null || echo "remind $id: exit $?" >&2
  done
```

A failed reminder prints its exit code and the loop moves on to the next
Learner. [exit-codes](exit-codes.md) says what each code means.

## Download every valid Certificate PDF

<!-- doctest: illustrative -->
```sh
mkdir -p pdfs
tmh certificates list --all --ndjson |
  jq -r 'select(.status == "valid") | .certificateId' |
  while read -r id; do
    [ -e "pdfs/$id.pdf" ] || tmh certificates download-pdf "$id" -o "pdfs/$id.pdf"
  done
```

Files already on disk are skipped, so a second run fetches only new
Certificates. [export-certificates.sh](../../examples/export-certificates.sh)
pairs this with a CSV export.

## Sync a roster CSV by External ID

<!-- doctest: illustrative -->
```sh
tail -n +2 roster.csv |
  while IFS=, read -r external_id email name; do
    tmh learners upsert-by-external-id "$external_id" --email "$email" --name "$name" > /dev/null
    case $? in
      0) ;;
      5) echo "skipped invalid row $external_id" >&2 ;;
      *) echo "stopped at $external_id" >&2; exit 1 ;;
    esac
  done
```

`roster.csv` has a header row and the columns `external_id,email,name`, with
no commas inside a field. Upserting by External ID creates and invites the
Learner on the first run and returns them on every run after, so the loop
is safe to repeat. Exit code 5 is a row the API rejected; anything else
stops the sync. [sync-roster.sh](../../examples/sync-roster.sh) also lists
Learners who are no longer on the roster.

## Update a Learner with its current ETag

<!-- doctest: illustrative -->
```sh
learner=sub_01JD80AB2C3D4E5F6G7H8J9K0M
etag=$(tmh learners retrieve "$learner" --show-headers 2>&1 >/dev/null |
  sed -n 's/^< Etag: //p')
tmh learners update "$learner" --name "Dana Whitfield" --if-match "$etag"
```

`--show-headers` writes the response headers to stderr. `2>&1 >/dev/null`
sends stderr into the pipe and drops the JSON. If the Learner changes
between the two commands, the update exits 4; run both again.

## Act on each bounced invite

<!-- doctest: illustrative -->
```sh
tmh events tail --types invite.bounced |
  while read -r occurred_at event_type event_id; do
    curl -fsS -X POST "$SLACK_WEBHOOK_URL" \
      -H 'content-type: application/json' \
      -d "{\"text\":\"$event_type at $occurred_at ($event_id)\"}"
  done
```

`tail` prints one line per Event on stdout, three columns separated by
spaces, and runs until Ctrl-C. Ctrl-C saves its place, so the next run
resumes after the last Event it handled.

## Gate a CI job on the compliance rate

<!-- doctest: illustrative -->
```sh
set -o pipefail
tmh compliance retrieve | jq -e '.compliantRate >= 0.95' > /dev/null || {
  echo "compliance under 95%, or tmh failed" >&2
  exit 1
}
```

`jq -e` exits 1 when the comparison is false. `pipefail` makes a `tmh`
failure (exit 2 for bad credentials, 7 for an outage) fail the step too. In
CI, set `TMH_CLIENT_ID` and `TMH_CLIENT_SECRET` in place of `tmh login`
([login](login.md#credentials-from-the-environment)).

## Check whether a Learner exists

<!-- doctest: recipe-not-found -->
```console
$ tmh learners retrieve sub_01JD8ZZZZZZZZZZZZZZZZZZZZZ
Error: Not found: No Learner with that id.
Request ID: req_01JD8Q
$ echo $?
3
```

<!-- doctest: illustrative -->
```sh
tmh learners retrieve "$id" > /dev/null 2>&1
case $? in
  0) echo "exists" ;;
  3) echo "no such Learner" ;;
  *) exit 1 ;;
esac
```

Exit code 3 means `404`. Branch on the code: the message text can change
between releases.

## Verify a Certificate without logging in

<!-- doctest: illustrative -->
```sh
tmh certificate-verifications retrieve a7Kq2Z |
  jq -e '.status == "valid"' > /dev/null && echo valid
```

Certificate verification is public: the command sends no Access token and
works on a machine that never ran `tmh login`. The tiny id is the last path
segment of the Certificate's `https://tme.so/<tinyId>` link.
