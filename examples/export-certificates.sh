#!/usr/bin/env bash
# Exports every Certificate in the Sandbox Organization to a CSV file,
# waiting for the export Operation to finish. With --pdfs, also downloads
# each Certificate's PDF into a directory next to it. Needs tmh, and jq for
# --pdfs.
#
#   export TMH_CLIENT_ID=cli_... TMH_CLIENT_SECRET=tmh_cs_...
#   examples/export-certificates.sh certificates.csv [--pdfs]
set -euo pipefail

: "${TMH_CLIENT_ID:?set TMH_CLIENT_ID to your Sandbox API client id}"
: "${TMH_CLIENT_SECRET:?set TMH_CLIENT_SECRET to your Sandbox API client secret}"
out="${1:?usage: export-certificates.sh <out.csv> [--pdfs]}"

# -o implies --wait; a large Organization's export can take longer than the
# default 5 minutes. A failed export exits with its problem's exit code.
tmh certificates export --format csv --wait --wait-timeout 15m -o "$out"

if [ "${2:-}" = "--pdfs" ]; then
  dir="${out%.csv}-pdfs"
  mkdir -p "$dir"
  tmh certificates list --all --ndjson |
    jq -r 'select(.status == "valid") | .certificateId' |
    while read -r certificate_id; do
      tmh certificates download-pdf "$certificate_id" -o "$dir/$certificate_id.pdf"
    done
fi
