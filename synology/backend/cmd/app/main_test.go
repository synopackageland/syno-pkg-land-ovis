package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/synopackageland/syno-pkg-land-sdk/api"
)

func TestRequestDeadlineFollowsTheBuildSetting(t *testing.T) {
	saved := requestTimeout
	t.Cleanup(func() { requestTimeout = saved })
	for setting, want := range map[string]time.Duration{
		"":    api.DefaultRequestTimeout,
		"120": 120 * time.Second,
		"1":   time.Second,
		"0":   api.DefaultRequestTimeout,
		"-5":  api.DefaultRequestTimeout,
		"x":   api.DefaultRequestTimeout,
	} {
		requestTimeout = setting
		var left time.Duration
		api.WithRequestDeadline(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			d, _ := r.Context().Deadline()
			left = time.Until(d)
		}), deadline()).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil))
		if left > want || left < want-time.Second {
			t.Errorf("requestTimeout %q: deadline in %v, want %v", setting, left, want)
		}
	}
}
