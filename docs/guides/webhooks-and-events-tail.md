# Webhooks and `events tail`

TeachMeHIPAA tells your systems what happens in your Organization with
**Events**, such as `learner.certified` or `invite.bounced`. Every Event is
kept in the Organization's event log for 30 days, and each Webhook
subscription receives the Event types it chose as signed **Deliveries**.

`tmh` reads the event log, and `tmh events tail --forward-to` turns your
machine into a webhook endpoint: it POSTs each new Event to your local
handler, signed like a real Delivery, so you can build and debug the handler
without a public URL.

## Reading the event log

`tmh events list` prints a page of the log, oldest first, and `--after`
continues after an Event id. `--type` lists one Event type, and `--all` reads to
the end:

<!-- doctest: events-list -->
```console
$ tmh events list --type learner.certified --all --ndjson
{"eventId":"evt_01JD8M","eventType":"learner.certified","orgId":"org_acme","occurredAt":"2026-11-15T11:58:00.000Z","data":{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K0M","email":"ana@example.com","externalId":"hr-123","courseId":"hipaa-2026","status":"compliant","occurredAt":"2026-11-15T11:58:00.000Z"}}
```

## Tailing Events

`tmh events tail` watches the log as Events happen and prints one line per
Event:

<!-- doctest: tail -->
```console
$ tmh events tail
Ready! Tailing Events; your local signing secret is 3q2+7wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= (^C to quit)
2026-11-15T12:00:00.000Z  learner.invited  evt_01JD8N
2026-11-15T12:00:04.000Z  learner.started  evt_01JD8P
```

- `tail` polls the event log every 2 seconds (`--interval`), backing off to
  10 seconds while it is quiet.
- It resumes where it last stopped for the profile and Organization, so a
  restart neither misses nor repeats Events. The first run starts with the
  next new Event. `--from-start` starts from the oldest Event in the log, and
  `--after <eventId>` starts after that Event.
- `--types learner.certified,learner.expired` shows only those Event types.
- Ctrl-C stops it and saves its place.

## Forwarding Events to your handler

With `--forward-to`, `tail` POSTs each Event to your local server and prints
the status your handler answered, so you see it fail:

<!-- doctest: tail-forward -->
```console
$ tmh events tail --forward-to http://localhost:3000/webhooks/tmh
Ready! Your local signing secret is 3q2+7wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= (^C to quit)
2026-11-15T12:00:00.000Z  learner.certified  evt_01JD8M  [200] POST http://localhost:3000/webhooks/tmh (12ms)
2026-11-15T12:00:04.000Z  learner.expired  evt_01JD8N  [500] POST http://localhost:3000/webhooks/tmh (8ms)
```

A handler failure or an unreachable server doesn't stop `tail`. To replay
one Event from the log while you fix a bug, resend it:

<!-- doctest: resend -->
```console
$ tmh events resend evt_01JD8N --forward-to http://localhost:3000/webhooks/tmh
2026-11-15T12:00:04.000Z  learner.expired  evt_01JD8N  [200] POST http://localhost:3000/webhooks/tmh (9ms)
```

The [webhook development loop example](../../examples/webhook-dev-loop.sh)
wraps this in a script.

## The local Signing secret

A forwarded Event is signed with your profile's **local Signing secret**
instead of a Webhook subscription's. It is created the first time you need
it, printed when `tail` starts, and stays the same across runs, so configure
your handler with it once. Each profile has its own, and `tmh logout` keeps
it. Print it with `--print-secret`:

<!-- doctest: print-secret -->
```console
$ tmh events tail --print-secret
3q2+7wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
```

<!-- doctest: illustrative -->
```sh
export TMH_WEBHOOK_SECRET="$(tmh events tail --print-secret)"
```

## Verifying forwarded Events

Your handler verifies forwarded Events with its SDK's `constructEvent`
(`construct_event` in Python and Ruby, `ConstructEvent` in .NET), unchanged
from production: pass the raw body, the headers, the secret and the request
path with its query string. For example, with the Node.js library and
Express:

```ts
app.post("/webhooks/tmh", express.raw({ type: "application/json" }), async (req, res) => {
  const event = await webhooks.constructEvent(
    req.body,
    req.headers,
    process.env.TMH_WEBHOOK_SECRET!,
    req.originalUrl,
  );
  console.log(event.eventType);
  res.sendStatus(200);
});
```

In production, use the Webhook subscription's Signing secret instead of the
local one. The SDKs reject a signature more than 5 minutes from their clock.
`tmh` signs each Event when it sends it, so even an old Event replayed with
`--from-start` or `resend` verifies.

### The local signature

A forwarded Event is signed the way TeachMeHIPAA signs a Delivery, with two
differences: the key is the local Signing secret, and `x-tmh-key-id` is
`local`.

The request is a `POST` with `content-type: application/json`. Its body is the
Event exactly as a Delivery carries it: JSON with every object's keys sorted.
`tmh` signs this canonical string with HMAC-SHA256, using the secret's UTF-8
bytes as the key:

```text
POST
<path and query of the --forward-to URL>
<timestamp>
<SHA-256 of the body, lowercase hex>
```

The lines are joined by `\n` with no trailing newline. The timestamp is the
send time in ISO 8601 UTC with milliseconds, such as
`2026-11-15T12:00:00.000Z`. For `--forward-to
http://localhost:3000/webhooks/tmh?source=cli`, the second line is
`/webhooks/tmh?source=cli`. The signature covers that path, not the host, so a
handler behind a proxy or in a container verifies as long as it sees the same
path. `tmh` doesn't follow redirects, since the signature wouldn't cover the
new path.

The headers are:

| Header | Value |
|---|---|
| `x-tmh-key-id` | `local` |
| `x-tmh-timestamp` | the timestamp in the canonical string |
| `x-tmh-body-hash` | the body's SHA-256, lowercase hex |
| `x-tmh-signature` | the HMAC-SHA256 of the canonical string, lowercase hex |
| `x-tmh-event-type` | the Event's type, such as `learner.certified` |
| `x-tmh-event-id` | the Event's id |

## Webhook subscriptions and Test deliveries

When your handler works, register its public URL as a Webhook subscription.
The response holds the subscription's Signing secret, which is shown only
once:

<!-- doctest: illustrative -->
```sh
tmh webhook-subscriptions create \
  --endpoint-url https://example.com/webhooks/tmh \
  --event-types learner.certified,learner.expired
```

`tmh webhooks trigger-test` (the same as `tmh webhook-subscriptions test`)
asks TeachMeHIPAA to send a real, signed Test delivery to a subscription's
endpoint. A Test delivery carries no Event, so it checks your endpoint and
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

`tmh webhook-subscriptions list`, `update`, `delete` and `redrive` manage
subscriptions; `redrive` retries failed Deliveries. See the
[resource reference](../reference/resources.md#tmh-webhook-subscriptions).
