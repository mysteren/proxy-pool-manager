---
name: proxy-tester
description: Test proxies for latency (TCP-connect), real usability (HTTP request through the proxy), download speed, and geo — with a concurrency semaphore, context cancellation, and progress events. Use in Milestones 3+ and when touching internal/services/tester.go.
---

# Proxy Tester

## When to use

- TCP-connect latency, HTTP usability confirmation, speed test, geo lookup.
- Batch checks (selected / non-working / unchecked / all) or cancellation.

## Key rules

- **Working proxy ≠ open port.** A proxy counts as working only when a light HTTP
  request passes *through* it. The full method spec (test protocol, validation URL,
  speed formula, timeouts, geо consensus) is in `docs/TESTING_STRATEGY.md` — follow
  it rather than re-deriving it here.
- Binding surface (`TesterService` in `internal/services/tester.go`): `TestProxy`,
  `TestProxies`, `TestAll`, `TestNonWorking`, `TestUnchecked`, `GetMyLocation`,
  `Cancel`. MTProto checking lives in `MTProtoService` (`TestUnchecked`, `LookupGeo`,
  `CancelGeo`).
- Use `net.Dialer.DialContext` (not `net.DialTimeout`) so cancellation aborts
  in-flight connections; HTTP requests carry the same `ctx`.
- Concurrency is `settings.test_concurrency` (semaphore or worker pool); speed runs
  under its own smaller limit.
- Store the batch `context.CancelFunc` in the service **under a mutex** — `Cancel()`
  is called from another goroutine.
- Emit progress/completion via typed events: `test:progress`, `test:completed`,
  `geo:progress`, `geo:completed` (registered in `main.go`).

## Pitfalls

- Goroutine leaks: every worker must `sem.Release(1)` and `wg.Done()` in `defer`.
- Do not block on an unbuffered channel with no reader; never `time.Sleep` to wait.
- SOCKS5 (`x/net/proxy`) needs a `DialContext` wrapper to honour cancellation.
- Do not skip the HTTP confirmation step — it prevents false "working" results from
  unrelated services listening on the port.
- On failure/cancel, clear stale metrics (`latency_ms`/`download_mbps = NULL`) rather
  than leaving the previous values.

## Pointers

- Full checking rules + testing strategy: `docs/TESTING_STRATEGY.md`.
- Settings keys/defaults, geo cache: `docs/DATA_MODEL.md`.
- Writing results: skill `proxy-storage`.
