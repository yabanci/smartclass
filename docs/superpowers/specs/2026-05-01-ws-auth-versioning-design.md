# WebSocket Auth + Contract Versioning — Design Spec

**Date:** 2026-05-01
**Topic:** Replace `?access_token=` query-string auth with single-use tickets, tighten WS CheckOrigin to the CORS allow-list, add `version` to every realtime event.
**Status:** Approved (ready for plan)
**Source:** `docs/superpowers/audits/2026-05-01-deep-audit.md` findings F-014, F-021, F-022.

## 1. Purpose

Three related WebSocket-layer issues from the audit:

- **F-014 (P3 Security/Observability).** WS upgrades currently accept a JWT via `?access_token=<jwt>`. Reverse proxies, browser history, telemetry pipelines all log URLs verbatim — every WS connection leaks a usable JWT into log sinks the JWT was never meant to reach.
- **F-021 (P3 Security).** `upgrader.CheckOrigin` returns `true` unconditionally. Defense-in-depth gap: the WS handshake doesn't verify the request came from an allowed origin.
- **F-022 (P3 Contracts).** `realtime.Event` has no `version` field, contradicting `pet/contracts.md` rule "every message carries a version field". When schema needs to change, consumers can't tell which version they're parsing.

This spec fixes all three. They cluster together because they're a single cross-service change touching backend WS handler + mobile WS provider, and shipping them together avoids two separate mobile rollouts.

## 2. Scope

**In scope**
- New endpoint `POST /api/v1/ws/ticket` (Bearer-authenticated) returning a 60-second single-use ticket.
- `TicketStore` interface + in-memory implementation (`sync.Map`-backed, expired-entry GC every minute).
- WS handler reads `?ticket=<x>`, calls `TicketStore.Consume`, then proceeds.
- `extractToken` in `middleware/auth.go` no longer reads `?access_token=` (Bearer header only).
- `upgrader.CheckOrigin` validates `Origin` header against `cfg.CORS.Origins`; empty origin (non-browser CLI/test) allowed.
- `realtime.Event` gains a `Version int` field; backend always emits `version: 1`.
- Mobile: new `WsEndpoints.createTicket()` call; `ws_provider.dart` fetches ticket then connects with `&ticket=<x>`.
- Tests: ~15 new (TicketStore, WS handshake variants, CheckOrigin matrix, Event version round-trip).

**Out of scope (with rationale)**
- Distributed `TicketStore` (Redis-backed, multi-instance). In-memory is correct for the single-process pet-project deploy; revisit when scaling out.
- Cookie-based auth fallback. Adds complexity for marginal benefit; mobile doesn't use cookies.
- WebSocket subprotocol-based token transport. Not portably supported in browsers.
- Replacing the access-JWT auth on regular HTTP endpoints (only the WS-specific query-string fallback).
- Per-message-deflate compression, reconnect-with-resume, or other WS protocol extensions.
- Versioning the *Topic* string (still `user:<uuid>:notifications`, `classroom:<uuid>:devices`, etc.).

## 3. Architecture

### Backend file map

```
backend/internal/realtime/ws/
├── ticket.go             NEW — TicketStore interface + in-memory impl + Cleanup goroutine
├── ticket_test.go        NEW — issue / consume-once / expire / replay / cleanup
├── handler.go            replace token-extract with ticket-validate; tighten CheckOrigin
├── handler_test.go       extend with new tests for ticket flow + CheckOrigin matrix
backend/internal/realtime/
├── event.go              + Version int field, default 1 (set wherever Event is built)
├── event_test.go         NEW — Version round-trip
backend/internal/server/server.go
                          register POST /api/v1/ws/ticket route under authenticated group
backend/cmd/server/main.go
                          construct *ws.MemTicketStore, pass into ws.Handler + ticket route
backend/internal/platform/httpx/middleware/auth.go
                          remove the `?access_token=` fallback — extractToken reads Bearer only
```

### Mobile file map

```
mobile/lib/core/api/endpoints/
├── ws_endpoints.dart (NEW)    — createTicket() returns String

mobile/lib/shared/providers/ws_provider.dart
                                URL builder: fetch ticket via WsEndpoints.createTicket(),
                                then build &ticket=<x> instead of &access_token=<jwt>;
                                reconnect path also re-fetches a fresh ticket.

mobile/lib/core/ws/ws_client.dart
                                no change — receives the final URL.
```

### TicketStore interface

```go
type Ticket struct {
    Raw       string    // the random base64 string the client uses
    UserID    uuid.UUID
    ExpiresAt time.Time
}

type TicketStore interface {
    Issue(ctx context.Context, userID uuid.UUID) (Ticket, error)
    Consume(ctx context.Context, raw string) (uuid.UUID, error) // marks used + returns userID
}

var ErrTicketUnknown = errors.New("ws: ticket unknown or already used")
```

In-memory implementation:
- `sync.Map` keyed on raw → `*ticketEntry{userID, expiresAt, used atomic.Bool}`
- `Issue`: generate 24 random bytes → base64 → store with 60s expiry.
- `Consume`: atomic load + check expired + atomic CompareAndSwap on `used` (false→true) → return userID. Single-use guarantee at the SQL-equivalent level (`atomic.Bool.CompareAndSwap`).
- Cleanup goroutine: every 60s, walk Map and delete entries past `expiresAt`. Bounded memory regardless of churn.

### Ticket endpoint

```
POST /api/v1/ws/ticket
Authorization: Bearer <jwt>
(empty body)

→ 200 OK
{
  "data": {
    "ticket": "Yk5y3...",       // 32-char base64
    "expiresAt": "2026-05-01T10:00:00Z"
  }
}
```

The endpoint sits inside the existing `authenticated` chi.Group in `server.go`, so the standard Authn middleware extracts the principal — no special-casing needed.

### WS handshake flow

```
mobile                        backend
------                        -------
POST /api/v1/ws/ticket
Authorization: Bearer <jwt>   --→ Authn middleware validates JWT
                              ←── 200 {ticket, expiresAt}

GET /api/v1/ws?ticket=<x>     --→ ws.Handler.Serve:
&topic=classroom:<id>:devices       1. extract ?ticket
                                    2. ticketStore.Consume(ticket) → principal
                                    3. authorizeTopics(principal, topics)
                                    4. CheckOrigin(r.Origin, cfg.CORS.Origins)
                                    5. upgrader.Upgrade
                              ←── 101 Switching Protocols
```

### CheckOrigin

```go
func makeCheckOrigin(allowedOrigins []string) func(*http.Request) bool {
    allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
    allowed := make(map[string]struct{}, len(allowedOrigins))
    for _, o := range allowedOrigins {
        allowed[strings.TrimSpace(o)] = struct{}{}
    }
    return func(r *http.Request) bool {
        if allowAll {
            return true
        }
        origin := r.Header.Get("Origin")
        if origin == "" {
            // No Origin header → not a browser. Mobile/CLI/tests reach here.
            // The ticket validation already gates these; allow.
            return true
        }
        _, ok := allowed[origin]
        return ok
    }
}
```

The `cfg.CORS.Origins` is plumbed into `ws.NewHandler` so the upgrader's CheckOrigin uses the same allow-list as CORS middleware. One config knob, two consistent behaviors.

### Event versioning

```go
// realtime/event.go
type Event struct {
    Version int            `json:"version"`     // always 1 for now; bump on schema break
    Topic   string         `json:"topic"`
    Type    string         `json:"type"`
    Payload map[string]any `json:"payload,omitempty"`
}
```

Wherever `realtime.Event{...}` is constructed (notification.publish, scene.Run, sensor publish, etc.) the literal must include `Version: 1`. Cleanest: a small constructor `realtime.NewEvent(topic, type, payload)` that always sets it.

## 4. Mobile changes

```dart
// mobile/lib/core/api/endpoints/ws_endpoints.dart (NEW)
class WsEndpoints {
  WsEndpoints(this._client);
  final ApiClient _client;

  /// Issues a 60-second single-use ticket for the next WebSocket upgrade.
  /// Caller must invoke this immediately before connecting (and on every
  /// reconnect — the ticket is one-shot).
  Future<String> createTicket() async {
    final res = await _client.post('/ws/ticket');
    return (res.data['ticket'] as String);
  }
}
```

```dart
// mobile/lib/shared/providers/ws_provider.dart (modified)
final wsConnectionProvider = ...; // existing

// In connectToClassroom:
final ticket = await WsEndpoints(apiClient).createTicket();
final url = '$baseWs/ws?topic=classroom:$classroomId:devices'
            '&topic=classroom:$classroomId:sensors'
            '&ticket=$ticket';
wsClient.connect(url);
```

The reconnect logic in `ws_client.dart` doesn't need to change — when it triggers a reconnect, the wrapping provider/UI calls `connectToClassroom` again, which fetches a new ticket. (If reconnect lives entirely inside `ws_client.dart` without going back through the provider, we hoist it: ws_client emits "needs reconnect", provider fetches ticket and re-connects.)

## 5. Backwards-compatibility

Pet project, single mobile client, single backend deploy. **Cut over cleanly:** ship backend + mobile in one PR/release. No transition window. The `?access_token=` query fallback is deleted in the same change that adds `?ticket=`.

If a user has an old mobile build cached when the backend rolls out, their WS connection fails until they update — acceptable for a pet-project pace.

## 6. Errors and codes

| Situation | HTTP/WS code | Body code |
|---|---|---|
| `POST /ws/ticket` without Bearer | 401 | `unauthorized` |
| WS upgrade missing `?ticket=` | 401 | `ws_ticket_required` |
| WS upgrade with unknown/expired/used ticket | 401 | `ws_ticket_invalid` |
| WS upgrade origin not in allow-list | 403 | `forbidden` |
| WS upgrade topic authz fails (F-020) | 403 | `forbidden` |

`ws_ticket_required` and `ws_ticket_invalid` are new domain error codes in `httpx/errors.go`.

## 7. Testing

### TicketStore (5 tests)
- `Issue_ReturnsRandomBase64` — two consecutive Issue calls produce distinct tickets; format matches `^[A-Za-z0-9_-]{32,}$`.
- `Consume_OnceOnly_SecondCallFails` — first Consume returns userID; second returns ErrTicketUnknown.
- `Consume_ExpiredFails` — clock-injection; advance past 60s; Consume returns ErrTicketUnknown.
- `Consume_UnknownTicket_Fails` — random string never issued → ErrTicketUnknown.
- `Cleanup_PrunesExpired` — issue 100; advance clock; trigger cleanup; map size drops to 0.

### Ticket endpoint (3 tests)
- `Issue_ValidBearer_200` — handler returns `{ticket, expiresAt}` with expiresAt ≈ now+60s.
- `Issue_NoBearer_401` — no auth header → 401.
- `Issue_WrongToken_401` — invalid JWT → 401 (via existing Authn middleware; sanity check).

### WS handshake (4 tests)
- `Upgrade_ValidTicket_101` — full happy path: issue, GET `/ws?ticket=<x>` → 101 Switching Protocols.
- `Upgrade_MissingTicket_401`.
- `Upgrade_ReusedTicket_401` — same ticket twice → second fails.
- `Upgrade_NoQueryAccessToken_NotAccepted` — passing `?access_token=<jwt>` (old format) → 401, even with a valid JWT. Proves the deprecated path is gone.

### CheckOrigin (3 tests)
- `Allowed_Origin_Succeeds` — origin in CORS list → upgrade succeeds.
- `Disallowed_Origin_Rejected` — not in list → 403.
- `Empty_Origin_Allowed` — no `Origin` header (CLI/mobile native) → upgrade succeeds.

### Event version (1 test)
- `Event_VersionRoundTrip` — marshal `Event{Version: 1, ...}` → unmarshal → Version == 1; missing-version JSON unmarshals as 0 (default).

**Total: 16 new tests.** All under `internal/realtime/ws/` and `internal/realtime/`.

## 8. Effort estimate

2-3 working days.
- Day 1: TicketStore + tests, ticket endpoint + route registration, server wiring.
- Day 2: WS handler refactor (consume ticket), CheckOrigin tightening, remove `?access_token=` fallback. Run regression.
- Day 3: Mobile changes (ws_endpoints.dart, provider URL build, smoke test), Event versioning + remaining tests, final regression.

## 9. Risks & mitigations

| Risk | Mitigation |
|---|---|
| In-memory TicketStore loses tickets on backend restart, breaking mid-flight upgrades | Tickets are 60s; client retries fetch a fresh one. Restart window is tiny in practice. Documented in spec. |
| Mobile reconnect path doesn't re-fetch ticket → infinite 401 loop | Reconnect MUST re-call `createTicket()` before each upgrade attempt. Specified in §4 and tested in mobile integration if available. |
| Event `Version` field breaks consumers that strict-decode JSON | Mobile parser already tolerates unknown fields (verified in audit). Backend deserializers (none currently) would need updates — but no backend code reads inbound Event JSON. |
| CheckOrigin breaks dev workflow when origin list is `["*"]` | Explicit `allowAll` shortcut keeps current dev-mode behavior. |
| Old mobile clients fail post-deploy | Pet project, intentional. Documented. |

## 10. Done criteria

- `curl http://localhost:8080/api/v1/ws/ticket` with valid Bearer returns `{data: {ticket, expiresAt}}`.
- `wscat -c "ws://localhost:8080/api/v1/ws?ticket=<x>"` connects on first use; second use of same ticket gets HTTP 401.
- `wscat -c "ws://localhost:8080/api/v1/ws?access_token=<jwt>"` returns 401 — the deprecated path is gone.
- Mobile `flutter run` connects WS successfully via the new flow.
- Every emitted event JSON includes `"version": 1`.
- 16 new tests pass under `-race`.
- All existing tests still pass; staticcheck/govulncheck/gosec/flutter analyze/flutter test all clean.

## 11. After this spec

`next-specs.md` queue after this one (in priority order):
- **Coverage v2** — push Tier 2 packages (analytics/sensor/device handlers) above 50%, ratchet CI gate.
- **Prometheus alert rules** — turn the metrics from the observability spec into actionable alerts.
- **Per-classroom configurability** (F-016, F-017) — per-classroom thresholds + timezone.
