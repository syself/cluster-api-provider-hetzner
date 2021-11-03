// Package utils implements some utility functions
package utils

import (
	"fmt"
	"strings"
)

// LabelsToLabelSelector is converting a map of labels to HCloud label
// selector.
func LabelsToLabelSelector(labels map[string]string) string {
	parts := make([]string, 0, len(labels))
	for key, val := range labels {
		parts = append(
			parts,
			fmt.Sprintf("%s==%s", key, val),
		)
	}
	return strings.Join(parts, ",")
}
