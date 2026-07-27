# 01 — LAN app shell + PIN gate

**What to build:** A **Go** process on the home PC uses **Fiber** to serve a mobile-friendly **HTMX** web UI reachable on the LAN. The learner opens it in a phone browser, enters a shared PIN, and is admitted only when the PIN is correct. Wrong PIN is clearly rejected. After unlock they see a simple authenticated home shell ready for later mining screens. Correct PIN establishes an **HTTP session cookie valid until process restart** (no TTL required in v1; **`HttpOnly` + `SameSite=Lax`**). This introduces the MiningApp facade and PinAuth port as the first real product seam.

**Stack (frozen for this feature):**

- Backend: **Go** + **Fiber**
- Frontend: **HTMX** (server-rendered HTML / partials; no SPA framework)
- MiningApp and ports: plain Go packages (no Fiber types inside domain)

**Also establish testing harness:** Go module, Fiber app entrypoint, template/static layout for HTMX, L1/L2/L3 test layout, and one documented command (e.g. `make test`) that runs the full automated suite. Later tickets only add tests to this harness.

**Blocked by:** None — can start immediately

**Status:** done

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [x] Go module + Fiber app boots and binds for LAN (or documented localhost path for dev)
- [x] HTMX available to the UI (CDN or vendored asset); PIN form and shell are server-rendered HTML
- [x] Server is reachable from another device on the same LAN (or documented localhost path for dev)
- [x] Shared PIN configuration is supported (env or local config; not hard-coded in source)
- [x] Correct PIN unlocks access and sets a session cookie that lasts until server restart
- [x] Session cookie is `HttpOnly` and `SameSite=Lax` (`Secure` not required for v1 LAN HTTP)
- [x] Incorrect PIN is rejected with a clear error and does not expose mining features
- [x] Restarting the server invalidates sessions (re-PIN required)
- [x] MiningApp exposes an unlock/auth use-case; Fiber handlers are a thin adapter over it

### Testing (required this ticket)

**L1 unit / facade (`go test`, no Fiber)**

- [x] `Unlock` (or equivalent) accept with correct PIN via fake PinAuth
- [x] Reject wrong PIN; no authenticated capability granted
- [x] Tests do not hard-code production PIN secret

**L2 HTTP smoke (Fiber `app.Test` or equivalent)**

- [x] Wrong PIN on unlock route → not authenticated (status + body assert)
- [x] Correct PIN → session cookie set with HttpOnly + SameSite=Lax; gated route returns success
- [x] Gated route without session → rejected

**L3 UI click smoke (headless browser → real HTMX UI)**

- [x] Open app → submit wrong PIN → error visible; mining shell not shown
- [x] Submit correct PIN → authenticated home shell visible
- [x] Stable selectors (`data-testid` or roles) on PIN form and shell

**Harness + gate**

- [x] Single documented command runs L1+L2+L3 (README or Makefile)
- [x] Full suite run green before ticket marked done
- [x] Manual note: LAN from phone (not automated)

### Done only when

- [x] All feature + testing checkboxes above
- [x] Full automated suite executed and green
