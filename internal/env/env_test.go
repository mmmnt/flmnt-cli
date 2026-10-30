package env

import "testing"

func TestGetReadsTheFlmntName(t *testing.T) {
	t.Setenv("FLMNT_SERVER_URL", "http://flmnt")
	t.Setenv("QUORUM_SERVER_URL", "")
	if got := Get("SERVER_URL"); got != "http://flmnt" {
		t.Fatalf("got %q", got)
	}
}

func TestGetPrefersTheFlmntNameOverTheQuorumName(t *testing.T) {
	t.Setenv("FLMNT_SERVER_URL", "http://flmnt")
	t.Setenv("QUORUM_SERVER_URL", "http://quorum")
	if got := Get("SERVER_URL"); got != "http://flmnt" {
		t.Fatalf("got %q", got)
	}
}

func TestGetFallsBackToTheQuorumNameWhenTheFlmntNameIsUnset(t *testing.T) {
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "http://quorum")
	if got := Get("SERVER_URL"); got != "http://quorum" {
		t.Fatalf("got %q", got)
	}
}

func TestGetIsEmptyWhenNeitherNameIsSet(t *testing.T) {
	t.Setenv("FLMNT_SERVER_URL", "")
	t.Setenv("QUORUM_SERVER_URL", "")
	if got := Get("SERVER_URL"); got != "" {
		t.Fatalf("got %q", got)
	}
}
