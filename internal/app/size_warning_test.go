package app

import (
	"testing"

	"github.com/lazypanic999/restivra/internal/verify"
)

func TestSizeDropWarning(t *testing.T) {
	cases := []struct {
		current int64
		earlier []int64
		want    string
	}{
		{30, []int64{100, 104, 98, 101}, "The backup is 70% smaller than the median of the previous 4 runs; it may be incomplete."},
		{50, []int64{100, 100, 100}, "The backup is 50% smaller than the median of the previous 3 runs; it may be incomplete."},
		{51, []int64{100, 100, 100}, ""},
		{10, []int64{100, 100}, ""},
		{10, []int64{100, 0, 100, -5}, ""},
		{0, []int64{100, 100, 100}, ""},
		{200, []int64{100, 100, 100}, ""},
	}
	for _, c := range cases {
		if got := sizeDropWarning(c.current, c.earlier); got != c.want {
			t.Errorf("sizeDropWarning(%d, %v) = %q, want %q", c.current, c.earlier, got, c.want)
		}
	}
}

func TestShrinkWarningFindsOnlySizeDrop(t *testing.T) {
	drop := sizeDropWarning(30, []int64{100, 100, 100})
	if shrinkWarning(&verify.Report{Warnings: []string{"The tested backup is identical to the previous run; backup freshness was not established.", drop}}) != drop {
		t.Fatal("size-drop warning not found")
	}
	if shrinkWarning(nil) != "" || shrinkWarning(&verify.Report{Warnings: []string{"No custom checks configured; application data was not validated."}}) != "" {
		t.Fatal("unrelated warning treated as size drop")
	}
}
