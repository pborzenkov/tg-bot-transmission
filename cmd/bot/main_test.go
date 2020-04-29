package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseArgs(t *testing.T) {
	var tests = []struct {
		name string
		args []string
		want *config
	}{
		{
			name: "empty",
			args: []string{},
			want: &config{
				TransmissionURL: "http://localhost:9091",
			},
		},
		{
			name: "all",
			args: []string{"-telegram.api-token", "abc", "-transmission.url", "http://example.com:1234", "-verbose"},
			want: &config{
				TelegramAPIToken: "abc",
				TransmissionURL:  "http://example.com:1234",
				Verbose:          true,
			},
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			conf, out, err := parseArgs("prog", tc.args)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if out != "" {
				t.Errorf("unexpected output: %q", err)
			}
			if !cmp.Equal(tc.want, conf) {
				t.Errorf("unexpected result, diff = \n%v", cmp.Diff(tc.want, conf))
			}
		})
	}
}
