# Plan: Fix mock/reconcile race with a RW-mutex gate

## Problem (two overlapping windows)

```
Test N BeforeEach                   Reconciler goroutine (test N)
─────────────────────────────────   ──────────────────────────────────
ResetAndCreateNamespace()
  └─ SetClients(fresh, unconfigured)   ← window A opens
                                         NewClient() → gets unconfigured mock
robotClient.On(...)                      mock.Method() → panic: no expectation
rescueSSHClient.On(...)              ← window A closes
```

Window B: test N finishes, `AssertExpectations` passes, BeforeEach of test N+1
starts — but an in-flight reconcile from test N still holds a reference to test
N's mocks and calls methods after `AssertExpectations`.

Both windows have the same root cause: reconcile goroutines are not synchronized
with the test setup/teardown cycle.

## Solution: ReconcileGate (RW-mutex)

A single `sync.RWMutex` (the *gate*) ties reconcile execution to mock setup:

- Every `Reconcile` call holds a **read lock** for its entire duration.
- Mock setup (create mocks → `On()` → `SetClients`) holds the **write lock**.

Write lock acquisition blocks until all in-flight reconciles finish.
While the write lock is held, no new reconcile can start.
When the write lock is released, all mocks are fully configured.
This closes both windows.

```
Test N+1 BeforeEach                 Reconciler goroutines
─────────────────────────────────   ──────────────────────────────────
gate.Lock()  ← blocks until all     [reconcile N finishes, releases RLock]
             in-flight reconciles    [no new reconciles can start]
             complete
create fresh mocks
robotClient.On(...)
rescueSSHClient.On(...)
SetClients(configured mocks)
gate.Unlock()                        [new reconciles start, see full mocks]
```

---

## Implementation steps

### Step 1 — Add `ReconcileGate` to affected reconcilers

The reconcilers that use mocked SSH/Robot clients are:
- `HetznerBareMetalHostReconciler` (SSH + Robot)
- `HCloudMachineReconciler` (SSH)

Add one field and a nil-guarded lock to each:

```go
// HetznerBareMetalHostReconciler in controllers/hetznerbaremetalhost_controller.go
type HetznerBareMetalHostReconciler struct {
    // ... existing fields ...
    ReconcileGate *sync.RWMutex // set by tests; nil in production
}

func (r *HetznerBareMetalHostReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
    if r.ReconcileGate != nil {
        r.ReconcileGate.RLock()
        defer r.ReconcileGate.RUnlock()
    }
    // ... existing body unchanged ...
}
```

Repeat for `HCloudMachineReconciler`.

In production the field is never set, so there is zero runtime overhead.

### Step 2 — Add `gate` to `ControllerResetter`

In `controllers/controllers_suite_test.go`:

```go
type ControllerResetter struct {
    gate                      sync.RWMutex  // ← new
    baremetalSSHClientFactory *mocks.SSHFactory
    // ... existing fields ...
}
```

In `NewControllerResetter`, wire the gate into the reconcilers:

```go
r := &ControllerResetter{...}
r.gate = sync.RWMutex{}
hetznerBareMetalHostReconciler.ReconcileGate = &r.gate
hcloudMachineReconciler.ReconcileGate = &r.gate
```

### Step 3 — Split mock setup: acquire write lock early, release late

**Current order (broken):**
```
ResetAndInitNamespace:
  1. create fresh mocks
  2. SetClients(unconfigured mocks)   ← factory exposed before On() calls
  return
BeforeEach:
  3. robotClient.On(...)              ← gap: factory served unconfigured mocks
  4. rescueSSHClient.On(...)
```

**New order:**

`ResetAndInitNamespace` (called inside `ResetAndCreateNamespace`):
1. Acquire `r.gate.Lock()` — blocks until all in-flight reconciles finish.
2. Create fresh mocks, call `.Test(t)` on each.
3. Store mocks on `testEnv` (the existing fields).
4. Do **not** call `SetClients` yet.
5. Update non-mock reconciler fields (namespace, HCloudClientFactory, etc.).

Add a new method `testEnv.CommitMockSetup()`:
1. Call `SetClients(configured mocks)` — factory now serves fully configured mocks.
2. Call `r.gate.Unlock()` — reconciles may resume.

Each test's `BeforeEach` calls `CommitMockSetup()` as the last setup step:

```go
BeforeEach(func() {
    testNs, err = testEnv.ResetAndCreateNamespace(ctx, "...")   // acquires gate write lock
    // ... create k8s objects ...

    robotClient = testEnv.RobotClient
    rescueSSHClient = testEnv.RescueSSHClient
    osSSHClient = testEnv.OSSSHClientAfterInstallImage

    robotClient.On("GetBMServer", mock.Anything).Return(...)
    // ... all On() calls ...
    configureRescueSSHClient(rescueSSHClient)

    testEnv.CommitMockSetup()   // ← new: calls SetClients + releases write lock
})
```

### Step 4 — Deadlock safety for panicking BeforeEach

If `BeforeEach` panics between `ResetAndCreateNamespace` and `CommitMockSetup`, the
write lock is never released and the suite hangs. Guard with `DeferCleanup`:

Inside `ResetAndCreateNamespace` (or `ResetAndInitNamespace`), after acquiring the lock:

```go
committed := false
DeferCleanup(func() {
    if !committed {
        testEnv.CommitMockSetup()   // release lock even on panic
    }
})
// expose `committed` so CommitMockSetup sets it to true
```

Or make `CommitMockSetup` idempotent with a `sync.Once`.

### Step 5 — Remove the now-redundant SSHFactory internal mutex

`SSHFactory.mu` was introduced to protect `NewClient` vs `SetClients` races.
With the gate held across the full reconcile + setup cycle, `SetClients` can only
run when no reconcile is active, so the internal mutex is no longer needed for
correctness. Keep it as defense-in-depth (concurrent `NewClient` calls from
multiple parallel reconciles are still safe) but update the comment.

---

## Files to change

| File | What changes |
|---|---|
| `controllers/hetznerbaremetalhost_controller.go` | Add `ReconcileGate *sync.RWMutex`; nil-guarded RLock at start of `Reconcile` |
| `controllers/hcloudmachine_controller.go` | Same |
| `controllers/controllers_suite_test.go` | Add `gate` to `ControllerResetter`; wire gate into reconcilers; split `ResetAndInitNamespace` (drop `SetClients` call) |
| `test/helpers/envtest.go` | Add `CommitMockSetup()` method; store pending gate + factory ref |
| `controllers/*_controller_test.go` (baremetal files) | Add `testEnv.CommitMockSetup()` at end of each `BeforeEach` |
| `pkg/services/baremetal/client/mocks/factory.go` | Update `SetClients` comment; internal mutex stays |

---

## Trade-offs

**Pro**
- Eliminates both race windows with one mechanism.
- The RW-mutex is familiar Go concurrency, easy to audit.
- Production reconcilers gain one nil-checked field — zero runtime cost when gate is nil.
- No changes to controller-manager setup or `SetupWithManager`.

**Con**
- Every `BeforeEach` must end with `CommitMockSetup()` — a new convention.
  Enforce with a comment in `ResetAndCreateNamespace` and a panic/fatalf if
  `CommitMockSetup` is never called before the next `ResetAndCreateNamespace`.
- The write lock serializes test-to-test transitions, but tests are already
  sequential in Ginkgo, so no throughput regression.
- Two reconciler structs gain a test-only field. This is a standard Go
  dependency-injection pattern and adds no production logic.
