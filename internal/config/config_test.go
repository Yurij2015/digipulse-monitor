package config

import (
	"os"
	"testing"
)

func TestResolveHeartbeatKey(t *testing.T) {
	t.Setenv("MONITOR_HEARTBEAT_KEY", "")
	t.Setenv("REDIS_PREFIX", "")

	cases := []struct {
		name       string
		heartbeat  string
		prefix     string
		want       string
		unsetHeart bool
	}{
		{
			name:      "explicit heartbeat key",
			heartbeat: "custom:heartbeat",
			want:      "custom:heartbeat",
		},
		{
			name:       "prefix fallback",
			prefix:     "laravel-database-",
			want:       "laravel-database-go_monitor:last_heartbeat",
			unsetHeart: true,
		},
		{
			name:       "no prefix local dev",
			unsetHeart: true,
			want:       "go_monitor:last_heartbeat",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.unsetHeart {
				_ = os.Unsetenv("MONITOR_HEARTBEAT_KEY")
			} else {
				t.Setenv("MONITOR_HEARTBEAT_KEY", tc.heartbeat)
			}
			t.Setenv("REDIS_PREFIX", tc.prefix)

			if got := resolveHeartbeatKey(); got != tc.want {
				t.Fatalf("resolveHeartbeatKey() = %q, want %q", got, tc.want)
			}
		})
	}
}
