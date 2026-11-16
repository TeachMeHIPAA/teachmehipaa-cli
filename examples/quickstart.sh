#!/usr/bin/env bash
# Prints the Sandbox Organization's Compliance summary and its first five
# Learners. Needs tmh and jq.
#
#   export TMH_CLIENT_ID=cli_... TMH_CLIENT_SECRET=tmh_cs_...
#   examples/quickstart.sh
set -euo pipefail

: "${TMH_CLIENT_ID:?set TMH_CLIENT_ID to your Sandbox API client id}"
: "${TMH_CLIENT_SECRET:?set TMH_CLIENT_SECRET to your Sandbox API client secret}"

tmh whoami

tmh compliance retrieve |
  jq -r '"\(.totals.compliant) of \(.totals.learners) Learners compliant (\(.compliantRate * 100 | floor)%)"'

echo
echo "First five Learners:"
tmh learners list --limit 5 |
  jq -r '.items[] | [.email, .status, (.certificate.expiresAt // "-")] | @tsv'
