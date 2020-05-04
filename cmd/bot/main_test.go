package main

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConfig(t *testing.T) {
	var tests = []struct {
		name string
		args []string
		env  []string
		want *config
	}{
		{
			name: "flags",
			args: []string{
				"-verbose",
				"-telegram.api-token", "abcde",
				"-telegram.allow-user", "user1",
				"-transmission.url", "http://example.com:1234",
			},
			want: &config{
				Verbose:         true,
				APIToken:        "abcde",
				AllowUser:       "user1",
				TransmissionURL: "http://example.com:1234",
			},
		},
		{
			name: "env",
			env: []string{
				"BOT_VERBOSE", "true",
				"BOT_TELEGRAM_API_TOKEN", "abcde",
				"BOT_TELEGRAM_ALLOW_USER", "user1",
				"BOT_TRANSMISSION_URL", "http://example.com:1234",
			},
			want: &config{
				Verbose:         true,
				APIToken:        "abcde",
				AllowUser:       "user1",
				TransmissionURL: "http://example.com:1234",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			unset := make([]string, 0)
			for i := 0; i < len(tc.env); i += 2 {
				unset = append(unset, tc.env[i])
				os.Setenv(tc.env[i], tc.env[i+1])
			}
			defer func() {
				for _, e := range unset {
					os.Unsetenv(e)
				}
			}()

			cfg := new(config)
			if err := cfg.command().Parse(tc.args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !cmp.Equal(tc.want, cfg) {
				t.Errorf("unexpected config, diff = \n%s", cmp.Diff(tc.want, cfg))
			}
		})
	}
}
