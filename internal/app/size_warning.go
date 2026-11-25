package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lazypanic999/restivra/internal/verify"
)

// sizeDropWarning flags an archive at most half the median size of earlier
// retained runs. Fewer than three earlier sizes is not a baseline. The run
// status is not changed: a smaller dump can be legitimate after a cleanup.
func sizeDropWarning(current int64, earlier []int64) string {
	sizes := make([]int64, 0, len(earlier))
	for _, size := range earlier {
		if size > 0 {
			sizes = append(sizes, size)
		}
	}
	if current <= 0 || len(sizes) < 3 {
		return ""
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i] < sizes[j] })
	mid := len(sizes) / 2
	median := float64(sizes[mid])
	if len(sizes)%2 == 0 {
		median = (float64(sizes[mid-1]) + float64(sizes[mid])) / 2
	}
	ratio := float64(current) / median
	if ratio > 0.5 {
		return ""
	}
	return fmt.Sprintf("The backup is %d%% smaller than the median of the previous %d runs; it may be incomplete.", int((1-ratio)*100+0.5), len(sizes))
}

const sizeDropMarker = "% smaller than the median of the previous "

// shrinkWarning returns the size-drop warning of a report, if any.
func shrinkWarning(report *verify.Report) string {
	if report == nil {
		return ""
	}
	return firstShrink(report.Warnings)
}

func firstShrink(warnings []string) string {
	for _, w := range warnings {
		if strings.HasPrefix(w, "The backup is ") && strings.Contains(w, sizeDropMarker) {
			return w
		}
	}
	return ""
}
