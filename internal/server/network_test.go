package server

import (
	"context"
	"slices"
	"testing"
)

func TestNetworkNoteReachesTheMenu(t *testing.T) {
	srv := testStation(t)
	if got := srv.lookAtNetwork(context.Background()); got != "" {
		t.Errorf("demo roasts need no network, got %q", got)
	}

	state := "signin"
	srv.netState.Store(&state)
	if !slices.Contains(srv.problemList(), networkNotes["signin"]) {
		t.Errorf("the menu should list the sign-in note, got %q", srv.problemList())
	}
}
