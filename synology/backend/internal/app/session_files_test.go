package app

import "testing"

func TestSessionFilesEnabledByTrustedBuildSetting(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{{"", false}, {"session", true}, {"yes", false}} {
		if got := SessionFilesEnabled(tc.value); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.value, got, tc.want)
		}
	}
}
