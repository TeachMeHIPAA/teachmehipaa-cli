#!/usr/bin/env bash
# Forwards the Sandbox Organization's Events to a webhook handler on your
# machine, signed like real Deliveries, while you develop it. Optionally
# starts the handler too, with TMH_WEBHOOK_SECRET set to the local Signing
# secret its constructEvent call should use. Press Ctrl-C to stop both.
#
#   export TMH_CLIENT_ID=cli_... TMH_CLIENT_SECRET=tmh_cs_...
#   examples/webhook-dev-loop.sh http://localhost:3000/webhooks/tmh -- node server.js
set -euo pipefail

: "${TMH_CLIENT_ID:?set TMH_CLIENT_ID to your Sandbox API client id}"
: "${TMH_CLIENT_SECRET:?set TMH_CLIENT_SECRET to your Sandbox API client secret}"
url="${1:-http://localhost:3000/webhooks/tmh}"
shift || true

# The local Signing secret stays the same across runs, so a handler
# configured with it once keeps verifying.
TMH_WEBHOOK_SECRET="$(tmh events tail --print-secret)"
export TMH_WEBHOOK_SECRET

if [ "${1:-}" = "--" ] && [ $# -gt 1 ]; then
  shift
  "$@" &
  handler=$!
  trap 'kill "$handler" 2>/dev/null || true' EXIT
else
  echo "Start your handler with TMH_WEBHOOK_SECRET=$TMH_WEBHOOK_SECRET" >&2
fi

# Send a few Events to see them arrive: invite a Sandbox Learner with
#   tmh learners create --email you+test@example.com
# or replay one from the log with
#   tmh events resend <eventId> --forward-to "$url"
tmh events tail --forward-to "$url"
