---
title: third-party-api-metrics
metatitle: Cluster API Provider Hetzner Third-Party API Call Metrics
sidebar: third-party-api-metrics
description: How CAPH instruments calls to the external hcloud and Robot APIs, and how to use that to debug which caph code is calling them
---

CAPH talks to two external Hetzner APIs:

* **hcloud** (`github.com/hetznercloud/hcloud-go`), used for HCloud machines, load balancers,
  networks, placement groups, ...
* **Robot** (`github.com/syself/hrobot-go`), used for bare metal hosts.

Both APIs are rate-limited. When a manager runs into rate limits, or simply calls an API more
often than expected, the practical debugging question is always the same: **which caph code is
making these calls, and how often?** This page documents the Prometheus metrics (and debug
logging) available to answer that, and lists known gaps for further improvement.

Background: [issue #2163](https://github.com/syself/cluster-api-provider-hetzner/issues/2163).

## Metrics reference

All metrics below are exposed on the manager's `/metrics` endpoint (`--metrics-bind-address`,
default `localhost:8080`) and registered on the controller-runtime metrics registry, so they show
up next to the usual controller-runtime/workqueue metrics.

| Metric | Labels | Always on? | Purpose |
| --- | --- | --- | --- |
| `hcloud_api_requests_total` | `code`, `method`, `api_endpoint` | yes | Generic per-endpoint hcloud call counter. Built into hcloud-go (`hcloud.WithInstrumentation`); `api_endpoint` is the unsubstituted path template hcloud-go used for the request (e.g. `/servers/%d`, not `/servers/1234`), so cardinality stays bounded regardless of fleet size. |
| `hcloud_api_request_duration_seconds` | `method` | yes | Latency histogram for hcloud calls, from the same library instrumentation. |
| `hcloud_api_in_flight_requests` | – | yes | Gauge of in-flight hcloud requests. |
| `caph_robot_requests_total` | `code`, `method`, `endpoint` | yes | Robot API equivalent of `hcloud_api_requests_total`. Added in this change — the Robot API previously had no metrics at all, only debug logging. Numeric path segments are collapsed the same way (e.g. `/server/1234` → `/server/N`). |
| `caph_robot_request_duration_seconds` | `method` | yes | Robot API equivalent of `hcloud_api_request_duration_seconds`. |
| `caph_robot_in_flight_requests` | – | yes | Robot API equivalent of `hcloud_api_in_flight_requests`. |
| `caph_hcloud_api_calls_by_caller_total` | `caller`, `method` | yes | **Every** hcloud `Client` method call, labeled by the caph Go function that called it (e.g. `pkg/services/hcloud/server.(*Service).findServer`) and the method name (`GetServer`, `CreateServer`, ...). This is the most direct way to answer "which caph code called hcloud". Cardinality is bounded by the number of call sites in the source code, not by fleet size. |
| `caph_robot_api_calls_by_caller_total` | `caller`, `method` | yes | Robot API equivalent of `caph_hcloud_api_calls_by_caller_total`. |
| `caph_hcloud_getserver_calls_by_bootstate_total` | `boot_state` | yes | `GetServer` calls broken down by the calling `HCloudMachine`'s `BootState` (or a fixed value like `remediation` for non-BootState callers). Bounded cardinality (small fixed set of BootState values). |
| `caph_robot_getbmserver_calls_by_state_total` | `provisioning_state` | yes | `GetBMServer` calls broken down by the calling `HetznerBareMetalHost`'s `ProvisioningState`. Bounded cardinality. |
| `caph_hcloud_getserver_calls_total` | `server_id` | only with `--metric-per-server-id` | `GetServer` calls broken down **per server ID**. One time series per distinct server, so this must not be left on permanently on a long-lived production manager — it's meant for bounded debugging runs (e.g. e2e tests). |
| `caph_robot_getbmserver_calls_total` | `server_id` | only with `--metric-per-server-id` | `GetBMServer` equivalent of the above. |

Implementation:

* Generic hcloud instrumentation: `pkg/services/hcloud/client/client.go` (`hcloud.WithInstrumentation`, from hcloud-go).
* Generic Robot instrumentation: `LoggingTransport.RoundTrip` in `pkg/services/baremetal/client/robot/robot_client.go`.
* Per-caller metrics: `recordAPICallByCaller` in the same two `client.go` files — called as the first line of every `Client` method.
* Per-BootState/ProvisioningState metrics: `RecordGetServerCallByBootState` (called from `pkg/services/hcloud/server/server.go` and `pkg/services/hcloud/remediation/remediation.go`) and `RecordGetBMServerCallByState` (called from `pkg/services/baremetal/host/host.go`).
* Per-server-ID metrics: inside `GetServer` / `GetBMServer` themselves, gated by `MetricPerServerID`.

### How the per-caller label is derived

`recordAPICallByCaller` uses `runtime.Caller(2)` / `runtime.FuncForPC` to read the Go function
name of whoever called the `Client` method (e.g. `GetServer`), and uses that as the `caller`
label — no manual bookkeeping needed at each call site, and no risk of a new call site being added
without instrumentation. This only works because it's called directly from the `realClient` /
`realHetznerRobotClient` method it instruments; if you add a layer of indirection there (e.g. a
generic decorator), the skip depth needs to move with it. Both `client.go` and `robot_client.go`
have a `TestRecordAPICallByCallerCapturesRealCaller` regression test pinning this down.

## Flags

| Flag | Applies to | Default | Effect |
| --- | --- | --- | --- |
| `--metric-per-server-id` | hcloud **and** Robot | `false` | Adds a `server_id` label to `GetServer`/`GetBMServer` call metrics (see table above). One Prometheus time series per distinct server ID — only enable for a bounded debugging run (e.g. e2e tests), never permanently on a long-lived production manager. |

`--metric-per-server-id` was originally added as `--hcloud-metric-per-server-id` (hcloud-only) and
was renamed and extended to also drive the Robot API metrics, since the same debugging need
(bounded per-server volume during a test run) applies to bare metal.

## How to debug "which caph code calls hcloud/Robot" with these metrics

To find which caph code is generating the most calls to either API right now:

```promql
topk(10, sum by (caller, method) (rate(caph_hcloud_api_calls_by_caller_total[5m])))
topk(10, sum by (caller, method) (rate(caph_robot_api_calls_by_caller_total[5m])))
```

To see if a specific reconcile phase is driving `GetServer`/`GetBMServer` volume:

```promql
sum by (boot_state) (rate(caph_hcloud_getserver_calls_by_bootstate_total[5m]))
sum by (provisioning_state) (rate(caph_robot_getbmserver_calls_by_state_total[5m]))
```

For a bounded run (e.g. reproducing a suspected per-server hot loop locally or in e2e), enable
`--metric-per-server-id` and look at `caph_hcloud_getserver_calls_total` /
`caph_robot_getbmserver_calls_total` to see if one particular server dominates. The e2e suite does
exactly this: it sets `--metric-per-server-id=true`, periodically scrapes `/metrics` from the
manager pod, and at the end of the run prints a table per server ID and per state for both APIs
(see `printThirdPartyAPICallsTables` in `test/e2e/e2e_suite_test.go`).

Robot API calls are also logged in full detail (method, URL, status code, calling stack) at log
level V(1) — run with `--log-level=debug` to see them.

## Known gaps / suggested follow-ups

Not implemented in this change, listed here so they don't get lost:

* **No object identity in metrics/logs.** The `caller` label identifies *which Go function* made
  a call, but not *which* `HCloudMachine` / `HetznerBareMetalHost` / `HetznerCluster` triggered it.
  For per-object investigation you still need to correlate with controller logs (which do include
  the reconciled object's name/namespace via the standard controller-runtime logger, but the
  hcloud/Robot API call logs themselves currently don't carry it through `LoggingTransport`).
  Passing the reconciled object's key through `ctx` and adding it as a log field in both
  `LoggingTransport`s would close this gap without adding label cardinality (log field, not a
  metric label).
* **No rate-limit-remaining metric.** Both APIs return rate-limit headers
  (`RateLimit-Remaining`/`RateLimit-Reset` style headers). Today CAPH only reacts *after* hitting a
  429 (`handleRobotRateLimitExceeded`, `hcloudutil.HandleRateLimitExceededV1Beta1`). Exposing the
  remaining-quota header as a gauge would give a leading indicator before requests start failing.
* **No dashboards/alerts yet.** The metrics above are new; a Grafana dashboard (per-caller top-N,
  per-endpoint request rate, rate-limit-remaining once added) and an alert for
  "hcloud/Robot API error rate elevated" would make this actionable without having to run ad hoc
  PromQL queries.
