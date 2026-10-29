# F-Common

## TODO

* [ ] add Caching

## Shared API setup

`setup.New` supplies recovery, content-type protection, explicit CORS origins and trusted proxy configuration (none by default). `MaxBodyBytes` optionally bounds request bodies; select a limit that accommodates uploads. `server.Run` supplies bounded headers and graceful shutdown. `baseHandler` readiness checks share a three-second request deadline. `buildinfo.Current` exposes semantic version, immutable commit and build timestamp; inject `F_VERSION`, `F_COMMIT`, and `F_BUILD_TIME`, or use Go linker values. API consumers should keep the existing `project-meta.jsonc` component versions and pair them with immutable image digests or `sha-<commit>` tags. Never substitute an undeployed local source version for live identity.

## Shared rate limiting

`ratelimit.FromEnv(service)` creates a bounded Redis-backed limiter. Install its
`Before()` middleware before authentication, set `ratelimit.Default`, and call
`ratelimit.User` only after a verified session identity is resolved. The shared
`authentication` middleware performs that call. `oauthsession` sends the optional
service-validation secret on its trusted identity-service request.

Configuration: `RATELIMIT_MODE=observe|enforce` (unset/disabled disables the feature),
`RATELIMIT_REDIS_ADDR`, `RATELIMIT_REDIS_PASSWORD`, `RATELIMIT_KEY`,
`RATELIMIT_EDGE_KEY`, `RATELIMIT_SERVICE_KEY`. Keys must be independently generated
secrets of at least 32 bytes. Only an authenticated edge may assert
`X-Fabkli-Client-IP`; other callers use their actual TCP peer. NGINX must overwrite
both this header and `X-Fabkli-Edge-Key`. Never trust a caller-supplied forwarded
chain or grant internal exceptions based solely on a pod/private address.

Authenticated traffic defaults to 1,200/minute/user and burst 120, configurable
with `RATELIMIT_AUTH_PER_MINUTE`/`RATELIMIT_AUTH_BURST`. Ordinary quotas are observed
first; sensitive login, registration and password/OAuth routes are enforced in
both modes. Login failure reservations are released only for the successful
attempt; existing failures remain. Forgot-password recipient reservation runs
before lookup or sending, covers unknown/provider accounts, and keeps suppressed
responses generic. Recipient keys are HMACs. All quota checks and spending are
atomic across replicas. Quotas expire; no account is permanently locked.

Store outages return 503 with Retry-After for sensitive operations. Ordinary APIs
fall back to bounded per-process buckets (weaker aggregate guarantees), with an
alert. Use `Engine.Report` for minute aggregate counters without identity labels.
HTTP clients must show 429/503 and retry delays, preserve form input, and avoid
automatically replaying writes or OAuth callback codes.
