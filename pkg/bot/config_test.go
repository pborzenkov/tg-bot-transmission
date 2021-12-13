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
			opts: []Option{WithAllowedUsers("user1", "user2")},
			want: &config{AllowedUsers: []string{"user1", "user2"}},
		},
		{
			name: "http_client",
			opts: []Option{WithHTTPClient(testHTTPClient)},
			want: &config{HTTPClient: testHTTPClient},
		},
		{
			name: "set_commands",
			opts: []Option{WithSetCommands()},
			want: &config{SetCommands: true},
		},
		{
			name: "locations",
			opts: []Option{
				WithLocations(Location{Name: "loc1", Path: "/download/loc1"}),
				WithLocations(
					Location{Name: "loc2", Path: "/download/loc2"},
					Location{Name: "loc3", Path: "/download/loc3"},
				),
			},
			want: &config{
				Locations: []Location{
					{Name: "loc1", Path: "/download/loc1"},
					{Name: "loc2", Path: "/download/loc2"},
					{Name: "loc3", Path: "/download/loc3"},
				},
			},
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
