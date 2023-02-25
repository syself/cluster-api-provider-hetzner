package cmd

import (
	"runtime"

	"github.com/tilt-dev/wmclient/pkg/analytics"
)

var Version string

func newAnalytics() (analytics.Analytics, error) {
	return analytics.NewRemoteAnalytics(
		"ctlptl",
		analytics.WithLogger(discardLogger{}),
		analytics.WithGlobalTags(globalTags()))
}

func globalTags() map[string]string {
	return map[string]string{
		"version": Version,
		"os":      runtime.GOOS,
	}
}

type discardLogger struct{}

func (dl discardLogger) Printf(fmt string, v ...interface{}) {}
