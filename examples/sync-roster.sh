#!/usr/bin/env bash
# Syncs a roster CSV into the Sandbox Organization: every row becomes a
# Learner, keyed by its External ID, and Learners no longer on the roster
# are listed for you to review. Needs tmh and jq.
#
# The CSV has a header row and the columns external_id,email,name, with no
# commas inside a field:
#
#   external_id,email,name
#   hr-123,ana@example.com,Ana Example
#
#   export TMH_CLIENT_ID=cli_... TMH_CLIENT_SECRET=tmh_cs_...
#   examples/sync-roster.sh roster.csv
set -euo pipefail

: "${TMH_CLIENT_ID:?set TMH_CLIENT_ID to your Sandbox API client id}"
: "${TMH_CLIENT_SECRET:?set TMH_CLIENT_SECRET to your Sandbox API client secret}"
roster="${1:?usage: sync-roster.sh <roster.csv>}"

synced=0
failed=0
on_roster=()
while IFS=, read -r external_id email name; do
  [ -n "$external_id" ] || continue
  name="${name%$'\r'}"
  on_roster+=("$external_id")
  # Upserting by External ID creates and invites the Learner the first time
  # and returns them every time after, so re-running the sync is safe.
  if learner="$(tmh learners upsert-by-external-id "$external_id" --email "$email" --name "$name")"; then
    synced=$((synced + 1))
    jq -r '"\(.externalId)\t\(.learnerId)\t\(.status)"' <<<"$learner"
  else
    status=$?
    # 4 is a conflict and 5 an invalid row: skip the row. Anything else,
    # such as an outage (7) or missing Scopes (2), stops the sync.
    if [ "$status" -ne 4 ] && [ "$status" -ne 5 ]; then
      exit "$status"
    fi
    failed=$((failed + 1))
    echo "skipped $external_id" >&2
  fi
done < <(tail -n +2 "$roster")

echo "Synced $synced Learners, skipped $failed." >&2

# Learners with an External ID that isn't on the roster any more. Review
# them, then remove each with: tmh learners offboard <learnerId>
tmh learners list --all --ndjson |
  jq -r --args '
    select(.externalId != null and .status != "removed")
    | select(.externalId as $id | $ARGS.positional | index($id) | not)
    | "not on roster\t\(.externalId)\t\(.learnerId)\t\(.email)"
  ' "${on_roster[@]+"${on_roster[@]}"}"
