package main

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pborzenkov/tg-bot-transmission/pkg/bot"
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
				"-telegram.allow-user", "user2",
				"-transmission.url", "http://example.com:1234",
				"-data.location", "loc1:/path/to/loc1",
				"-data.location", "loc2:/path/to/loc2",
			},
			want: &config{
				Verbose:         true,
				APIToken:        "abcde",
				AllowUsers:      []string{"user1", "user2"},
				TransmissionURL: "http://example.com:1234",
				Locations: []bot.Location{
					{Name: "loc1", Path: "/path/to/loc1"},
					{Name: "loc2", Path: "/path/to/loc2"},
				},
			},
		},
		{
			name: "env",
			env: []string{
				"BOT_VERBOSE", "true",
				"BOT_TELEGRAM_API_TOKEN", "abcde",
				"BOT_TELEGRAM_ALLOW_USER", "user1,user2",
				"BOT_TRANSMISSION_URL", "http://example.com:1234",
				"BOT_DATA_LOCATION", "loc1:/path/to/loc1,loc2:/path/to/loc2",
			},
			want: &config{
				Verbose:         true,
				APIToken:        "abcde",
				AllowUsers:      []string{"user1", "user2"},
				TransmissionURL: "http://example.com:1234",
				Locations: []bot.Location{
					{Name: "loc1", Path: "/path/to/loc1"},
					{Name: "loc2", Path: "/path/to/loc2"},
				},
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
