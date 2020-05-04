package bot

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConfig(t *testing.T) {
	testLogger := noopLogger{}

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
			name: "allowed_users",
			opts: []Option{WithAllowedUser("user1")},
			want: &config{AllowedUser: "user1"},
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
