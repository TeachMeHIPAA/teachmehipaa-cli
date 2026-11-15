# Standalone Go binary; `events tail` replaces `listen`

`tmh` is a Go binary (cobra, GoReleaser) that talks HTTP directly instead of
wrapping one of the SDKs, as the Stripe CLI does with stripe-go. A
Bun-compiled binary would be 60–100 MB with weaker keyring support. There is
no server-side relay like Stripe's `listen` websocket, so
`tmh events tail --forward-to` polls the Organization's event log. It
re-signs each Event locally with a per-profile Signing secret, computing the
signature for the forward-to URL's path. Partners can then verify forwarded
Events with their own SDK's `constructEvent`.
