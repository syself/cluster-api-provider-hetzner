# Alternative solutions to the mock/reconcile race

The problem to solve: reconciler goroutines from test N can call mocked client
methods (a) while test N+1 is in the middle of setting up new mocks (window A),
or (b) after `AssertExpectations` has already run for test N (window B).

The [plan.md](plan.md) proposes a RW-mutex gate where every `Reconcile` holds a
read lock and mock setup holds a write lock. Below are the alternatives
considered and why each was accepted or rejected.

---

## A. WaitGroup drain (incomplete)

Add a `sync.WaitGroup` to `ControllerResetter`. Each `Reconcile` calls
`wg.Add(1)` / `wg.Done()`. `ResetAndInitNamespace` calls `wg.Wait()` before
swapping mocks.

```go
func (r *HetznerBareMetalHostReconciler) Reconcile(ctx, req) (Result, error) {
    r.wg.Add(1)
    defer r.wg.Done()
    // ...
}
// test setup:
r.wg.Wait()          // drain in-flight reconciles
r.SetClients(...)    // now safe to swap
robotClient.On(...)  // configure expectations
```

**Fixes:** window B (post-assert calls) — in-flight reconciles from test N
finish before mocks change.

**Does not fix:** window A (setup gap). After `wg.Wait()` returns, new reconciles
can start immediately and call `NewClient` before `On()` is registered. WaitGroup
drains; it does not exclude concurrent readers.

**Verdict:** partial fix only. Would need a second mechanism (e.g. a mutex
around the `SetClients` + `On()` block) to close window A — at which point it
becomes the RW-mutex plan with extra steps.

---

## B. Configure mocks via callback before SetClients

Instead of a gate, change the call site: pass a setup callback into
`ResetAndCreateNamespace`. The callback configures `On()` expectations; only
then is `SetClients` called. The factory never exposes an unconfigured mock.

```go
// hypothetical API
testNs, err = testEnv.ResetAndCreateNamespace(ctx, "ns", func(r *robotmock.Client, ssh *sshmock.Client) {
    r.On("GetBMServer", mock.Anything).Return(...)
    ssh.On("Reboot", ...).Return(...)
})
// CommitMockSetup called internally before returning
```

**Pros:**
- No production code changes; no mutex.
- Window A is closed because `SetClients` is never called with unconfigured mocks.

**Does not fix:** window B. A reconcile from test N that finishes after
`AssertExpectations` has been called will still call methods on test N's mocks
(now held only by the goroutine, not by the factory). The WaitGroup drain would
still be needed alongside this.

**Also:** requires restructuring every `BeforeEach` into a callback signature,
which is a large churn across all test files and makes it awkward to reference
local variables (`machineName`, `hetznerCluster`, etc.) that are declared in the
outer `Describe` scope.

**Verdict:** cleaner for window A, but requires the WaitGroup drain for window B,
and imposes an awkward callback API on all test files. The RW-mutex covers both
windows in one mechanism.

---

## C. Per-namespace client factories

Give each test namespace its own set of mock clients. The factory holds a
`map[namespace]*mocks` and returns the right mock based on the reconcile
request's namespace. Test N and test N+1 use different namespaces (already the
case), so their mocks never collide.

**Pros:**
- No synchronization primitives.
- Works for window A: a reconcile from test N always reads test N's namespace
  entry, never test N+1's.

**Problems:**
- Requires refactoring the `SSHFactory` and `RobotFactory` interfaces to accept
  a namespace parameter, which propagates into production code (`NewClient`
  currently takes `sshclient.Input`, not a namespace).
- Does not fix window B for the same namespace (e.g. `AfterEach` deletes
  objects, triggering a reconcile on the same test-N namespace that sees
  test N's already-asserted mocks).
- Does not handle `HCloudClientFactory`, which is replaced wholesale by pointer
  swap in `ResetAndInitNamespace` and carries its own version of the same race.

**Verdict:** large refactor with incomplete coverage. Not worth it.

---

## D. Goroutine-safe custom fakes instead of testify mocks

Replace `sshmock.Client` and `robotmock.Client` (testify generated) with
hand-written fakes that:
- Store configured responses in a mutex-protected map.
- Return zero values for calls with no configured response instead of panicking.
- Record all received calls for later assertion.

```go
type SafeSSHClient struct {
    mu       sync.Mutex
    returns  map[string]sshclient.Output
    calls    []string
}
func (c *SafeSSHClient) Reboot() sshclient.Output {
    c.mu.Lock(); defer c.mu.Unlock()
    c.calls = append(c.calls, "Reboot")
    if r, ok := c.returns["Reboot"]; ok { return r }
    return sshclient.Output{}  // zero value, no panic
}
```

**Pros:**
- No race condition possible — unexpected calls return zero, not panic.
- Thread-safe by construction.
- No production code changes.

**Cons:**
- Eliminates testify's assertion model entirely; have to write `AssertCalled` /
  call-count checks manually.
- Silently swallows unexpected calls; bugs that testify would surface as test
  failures become silent, producing wrong behavior that may not be caught.
- The SSH and Robot client interfaces have many methods — substantial code to
  write and maintain.

**Verdict:** changes the testing philosophy too much and trades assertion
visibility for safety. Acceptable if all else fails, but a last resort.

---

## E. Stop and restart the controller-manager between tests

In `AfterEach`, cancel the manager's context and call `wg.Wait()` until all
goroutines exit. In `BeforeEach`, start a new manager.

**Pros:**
- Perfect isolation: no goroutine from test N survives into test N+1.
- No synchronization primitives needed in mock setup.

**Cons:**
- Controller-manager startup is slow (API server watch setup, leader election,
  webhook handshake). Typically several seconds per manager.
- The integration test suite has many tests; a per-test manager restart would
  multiply total runtime by an order of magnitude.
- Teardown is also slow: graceful shutdown waits for in-flight reconciles with
  a timeout.

**Verdict:** correct but unacceptably slow. Use only if targeted
synchronization proves too complex to maintain.

---

## F. controller-runtime WithOptions override (no production-code changes)

controller-runtime builder allows setting the reconciler via `WithOptions`:

```go
// from doController in controller-runtime/pkg/builder/controller.go
if ctrlOptions.Reconciler != nil && r != nil {
    return errors.New("reconciler was set via WithOptions() and via Build() or Complete()")
}
```

If `Options.Reconciler` is set to the gated wrapper and `Complete(nil)` is
called, the builder uses the options-provided reconciler. This means in the test
suite we would not call `SetupWithManager` but instead replicate its builder
setup (the `For()`, `Watches()`, `WithPredicates()` calls) and swap in the
wrapper:

```go
gate := &sync.RWMutex{}
err := ctrl.NewControllerManagedBy(testEnv).
    WithOptions(controller.Options{Reconciler: &helpers.ReconcileGate{
        Inner: hetznerBareMetalHostReconciler,
        Mu:    gate,
    }}).
    For(&infrav1.HetznerBareMetalHost{}).
    // must duplicate all Watches / predicates from SetupWithManager here
    Complete(nil)
```

**Pros:**
- No field added to production reconciler structs.

**Cons:**
- Must duplicate all watches and predicates from each reconciler's
  `SetupWithManager` in the test suite. This is brittle: any change to
  `SetupWithManager` (adding a watch, changing a predicate) must be mirrored in
  the test suite setup; divergence silently breaks coverage.
- The plan's approach adds one nil-checked field — a much smaller surface area.

**Verdict:** avoids one small production-code change at the cost of a much
larger and more fragile test-code coupling. Not worth it.

---

## G. "No-op sentinel" client during setup gap

Instead of blocking new reconciles during mock setup, the factory returns a
no-op sentinel client during the gap:

```go
func (f *SSHFactory) NewClient(in sshclient.Input) sshclient.Client {
    f.mu.RLock(); defer f.mu.RUnlock()
    if f.sentinelMode {
        return &noopSSHClient{}  // returns zero values, no panic
    }
    // ... return real mock
}
```

`ResetAndInitNamespace` sets `sentinelMode = true`, configures mocks, calls
`SetClients`, then sets `sentinelMode = false`.

**Pros:**
- No production code changes; no gate in reconciler.

**Cons:**
- A reconcile that gets the no-op client during setup will proceed through state
  machine transitions with fake zero-value responses. This can corrupt the
  Kubernetes object state in ways that affect the test that's about to run.
- Does not fix window B at all (post-assert calls still panic on the old mocks).
- Adds complexity to `SSHFactory` with a mode switch.

**Verdict:** narrows window A's severity from "panic" to "silent wrong behavior",
which may be worse. Rejected.

---

## Summary

| Alternative | Fixes window A | Fixes window B | Production change | Complexity |
|---|---|---|---|---|
| **RW-mutex gate (plan)** | ✓ | ✓ | 1 field + 3 lines per reconciler | Low |
| WaitGroup drain | ✗ | ✓ | 1 field + 2 lines per reconciler | Low |
| Callback before SetClients | ✓ | ✗ | None | Medium (API churn) |
| Per-namespace factories | ✓ (partial) | ✗ | Large refactor | High |
| Goroutine-safe fakes | ✓ | ✓ | None | High (rewrite mocks) |
| Stop/restart manager | ✓ | ✓ | None | Low (but very slow) |
| WithOptions trick | ✓ | ✓ | None | High (brittle duplication) |
| No-op sentinel client | ✓ (weakly) | ✗ | None | Medium |

The RW-mutex gate is the only option that closes both windows, requires minimal
production code changes, and adds no test-suite brittleness. The next-closest
alternative — goroutine-safe fakes — would also work but changes the testing
model and hides bugs rather than surfacing them.
