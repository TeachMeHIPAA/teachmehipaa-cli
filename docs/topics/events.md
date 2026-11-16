# Events and webhooks

<!-- doctest: synopsis -->
```text
tmh events list [--type <eventType>] [--after <eventId>] [--all [--ndjson]]
tmh events tail [--types <type,...>] [--from-start | --after <eventId>] [--interval <duration>]
tmh events tail --forward-to <url>
tmh events tail --print-secret
tmh events resend <eventId> --forward-to <url>
tmh webhook-subscriptions create --endpoint-url <url> --event-types <type,...>
tmh webhooks trigger-test <subscriptionId> [--wait]
```

An **Event** records something that happened in your Organization, such as
`learner.certified` or `invite.bounced`. The Organization's event log keeps
each Event for 30 days. A Webhook subscription receives the Event types it
chose as signed **Deliveries**.

`tmh events tail --forward-to` stands in for TeachMeHIPAA on your machine:
it POSTs each new Event to a local handler, signed like a Delivery. You
build and debug the handler with no public URL.

## events list

`tmh events list` prints a page of the log, oldest first. `--after`
continues after an Event id, `--type` keeps one Event type, and `--all`
reads to the end:

<!-- doctest: events-list -->
```console
$ tmh events list --type learner.certified --all --ndjson
{"eventId":"evt_01JD8M","eventType":"learner.certified","orgId":"org_acme","occurredAt":"2026-11-15T11:58:00.000Z","data":{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K0M","email":"ana@example.com","externalId":"hr-123","courseId":"hipaa-2026","status":"compliant","occurredAt":"2026-11-15T11:58:00.000Z"}}
```

## events tail

`tmh events tail` watches the log and prints a line per Event on stdout:

<!-- doctest: tail -->
```console
$ tmh events tail
Ready! Tailing Events; your local signing secret is 3q2+7wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= (^C to quit)
2026-11-15T12:00:00.000Z  learner.invited  evt_01JD8N
2026-11-15T12:00:04.000Z  learner.started  evt_01JD8P
```

| Behavior | Detail |
|---|---|
| Polling | Every 2 seconds (`--interval`), backing off to 10 seconds while the log is quiet. |
| Resume | Picks up where the last `tail` stopped for this profile and Organization, so a restart misses and repeats nothing. The first run starts at the next new Event. |
| Start point | `--from-start` starts at the oldest Event in the log; `--after <eventId>` starts after that Event. |
| Filter | `--types learner.certified,learner.expired` prints only those types. |
| Stop | Ctrl-C saves the position and exits. |

The `Ready!` line goes to stderr. stdout gets only the Event lines, in
three columns separated by two spaces, ready for `while read` (see
[recipes](recipes.md#act-on-each-bounced-invite)).

## --forward-to

With `--forward-to`, `tail` POSTs each Event to your server and adds the
status your handler returned:

<!-- doctest: tail-forward -->
```console
$ tmh events tail --forward-to http://localhost:3000/webhooks/tmh
Ready! Your local signing secret is 3q2+7wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= (^C to quit)
2026-11-15T12:00:00.000Z  learner.certified  evt_01JD8M  [200] POST http://localhost:3000/webhooks/tmh (12ms)
2026-11-15T12:00:04.000Z  learner.expired  evt_01JD8N  [500] POST http://localhost:3000/webhooks/tmh (8ms)
```

A `500` from your handler or a refused connection doesn't stop `tail`. To
replay one Event while you fix the bug:

<!-- doctest: resend -->
```console
$ tmh events resend evt_01JD8N --forward-to http://localhost:3000/webhooks/tmh
2026-11-15T12:00:04.000Z  learner.expired  evt_01JD8N  [200] POST http://localhost:3000/webhooks/tmh (9ms)
```

[webhook-dev-loop.sh](../../examples/webhook-dev-loop.sh) starts your
handler and the forwarder together.

## The local Signing secret

Forwarded Events are signed with the profile's **local Signing secret**, not
a Webhook subscription's. `tmh` creates it on first use, prints it when
`tail` starts, and keeps it across runs, so you configure your handler once.
Each profile has its own, and `tmh logout` leaves it in place.

<!-- doctest: print-secret -->
```console
$ tmh events tail --print-secret
3q2+7wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
```

<!-- doctest: illustrative -->
```sh
export TMH_WEBHOOK_SECRET="$(tmh events tail --print-secret)"
```

Your handler verifies a forwarded Event with the same SDK call it uses in
production (`constructEvent` in Node.js and Java, `construct_event` in
Python and Ruby, `ConstructEvent` in .NET). Pass it the raw body, the
headers, the secret and the request path with its query string. In
production, swap the local secret for the subscription's. The SDKs reject a
timestamp more than 5 minutes from their clock; `tmh` signs at send time, so
an old Event replayed by `--from-start` or `resend` still verifies.

### Signature format

A forwarded Event is signed the way TeachMeHIPAA signs a Delivery, with two
differences: the key is the local Signing secret, and `x-tmh-key-id` is
`local`.

The request is a `POST` with `content-type: application/json`. The body is
the Event as a Delivery carries it: JSON with every object's keys sorted.
`tmh` signs this canonical string with HMAC-SHA256, keyed by the secret's
UTF-8 bytes:

```text
POST
<path and query of the --forward-to URL>
<timestamp>
<SHA-256 of the body, lowercase hex>
```

The lines are joined by `\n` with no trailing newline. The timestamp is the
send time in ISO 8601 UTC with milliseconds (`2026-11-15T12:00:00.000Z`).
For `--forward-to http://localhost:3000/webhooks/tmh?source=cli`, the second
line is `/webhooks/tmh?source=cli`. The host isn't signed, so a handler
behind a proxy or in a container verifies as long as it sees the same path.
`tmh` doesn't follow redirects, since the signature wouldn't cover the new
path.

| Header | Value |
|---|---|
| `x-tmh-key-id` | `local` |
| `x-tmh-timestamp` | the timestamp in the canonical string |
| `x-tmh-body-hash` | the body's SHA-256, lowercase hex |
| `x-tmh-signature` | the HMAC-SHA256 of the canonical string, lowercase hex |
| `x-tmh-event-type` | the Event's type, such as `learner.certified` |
| `x-tmh-event-id` | the Event's id |

## Webhook subscriptions

Once the handler works, register its public URL. The response carries the
subscription's Signing secret, and the API shows it only this once:

<!-- doctest: illustrative -->
```sh
tmh webhook-subscriptions create \
  --endpoint-url https://example.com/webhooks/teachmehipaa \
  --event-types learner.certified,learner.expired | jq -r .secret
```

`tmh webhooks trigger-test` (also `tmh webhook-subscriptions test`) asks
TeachMeHIPAA to send a real, signed Test delivery to the subscription's
endpoint. A Test delivery carries no Event; it proves the endpoint and the
verifier end to end:

<!-- doctest: trigger-test -->
```console
$ tmh webhooks trigger-test whs_01JD82EF3G4H5J6K7M8N9P0Q1R --wait
{
  "name": "organizations/org_acme/operations/op_01JD86",
  "done": true,
  "metadata": {
    "kind": "webhookSubscriptions.test"
  },
  "response": {
    "_tag": "sent",
    "status": 204
  }
}
```

`list`, `update` and `delete` manage subscriptions, and `redrive` retries
failed Deliveries. See the
[resource reference](../reference/resources.md#tmh-webhook-subscriptions).
