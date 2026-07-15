package hcloudclient

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// fakeGetServerMethod mimics realClient.GetServer's shape: a method that calls
// recordAPICallByCaller directly. businessLogicCaller mimics the caph business logic that calls
// such a Client method, so the call chain has the same depth as production
// (findServer -> GetServer -> recordAPICallByCaller): two hops away from recordAPICallByCaller.
// This pins down the runtime.Caller(2) skip depth in recordAPICallByCaller against silent breakage,
// e.g. if someone adds an indirection layer without adjusting it.
func fakeGetServerMethod() {
	recordAPICallByCaller("GetServer")
}

func businessLogicCaller() {
	fakeGetServerMethod()
}

func TestRecordAPICallByCallerCapturesRealCaller(t *testing.T) {
	businessLogicCaller()

	metricCh := make(chan prometheus.Metric, 100)
	apiCallsByCallerTotal.Collect(metricCh)
	close(metricCh)

	const wantCaller = "pkg/services/hcloud/client.businessLogicCaller"
	found := false
	for m := range metricCh {
		var d dto.Metric
		if err := m.Write(&d); err != nil {
			t.Fatal(err)
		}
		for _, lp := range d.Label {
			if lp.GetName() == "caller" && lp.GetValue() == wantCaller {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected caller label %q to be recorded on apiCallsByCallerTotal", wantCaller)
	}
}
