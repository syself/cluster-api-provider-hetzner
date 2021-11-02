package utils

import (
	"fmt"
	"strings"
)

// LabelsToLabelSelector is converting a map of labels to HCloud label
// selector.
func LabelsToLabelSelector(labels map[string]string) string {
	var parts []string
	for key, val := range labels {
		parts = append(
			parts,
			fmt.Sprintf("%s==%s", key, val),
		)
	}
	return strings.Join(parts, ",")
}
