package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pborzenkov/tg-bot-transmission/pkg/bot"
)

func TestLocations(t *testing.T) {
	var locations []bot.Location

	fl := newLocationsValue(&locations)
	for _, s := range []string{"loc1:/path/to/loc1", "loc2:/path/to/loc2"} {
		if err := fl.Set(s); err != nil {
			t.Fatalf("unexpected error setting %q: %v", s, err)
		}
	}

	want := []bot.Location{
		{Name: "loc1", Path: "/path/to/loc1"},
		{Name: "loc2", Path: "/path/to/loc2"},
	}
	if diff := cmp.Diff(want, locations); diff != "" {
		t.Errorf("unexpected result (-want +got):\n%s", diff)
	}

	if want, got := "loc1:/path/to/loc1,loc2:/path/to/loc2", fl.String(); want != got {
		t.Errorf("unexpected string representation, want = %q, got = %q", want, got)
	}
}
