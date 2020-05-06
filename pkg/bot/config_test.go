package bot

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConfig(t *testing.T) {
	testLogger := noopLogger{}
	testHTTPClient := &http.Client{}

	var tests = []struct {
		name string
		opts []Option
		want *config
	}{
		{
			name: "logger",
			opts: []Option{WithLogger(testLogger)},
			want: &config{Log: testLogger},
		},
		{
			name: "allowed_user",
			opts: []Option{WithAllowedUser("user1")},
			want: &config{AllowedUser: "user1"},
		},
		{
			name: "http_client",
			opts: []Option{WithHTTPClient(testHTTPClient)},
			want: &config{HTTPClient: testHTTPClient},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := new(config)
			for _, opt := range tc.opts {
				opt.apply(cfg)
			}
			if !cmp.Equal(tc.want, cfg) {
				t.Errorf("got unexpected result, diff = \n%s", cmp.Diff(tc.want, cfg))
			}
		})
	}
}
