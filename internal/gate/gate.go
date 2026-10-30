package gate

import (
	"time"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
	"github.com/mmmnt/flmnt-cli/internal/session"
)

const injectionText = `[flmnt reminder] No recent keyframe found for this session.
Before starting your next task, call write_keyframe on this session's keyframe stream to checkpoint your current understanding.
Use list_streams to find the stream_id if needed.`

// Config is the context-recency check. GQL is the authenticated router GraphQL client so the keyframe
// read is scoped to the caller's workspace; SessionID names the agent session whose keyframe stream,
// {ProjectID}::{SessionID}::keyframe, is read. A stale/missing keyframe triggers the reminder.
type Config struct {
	GQL       *apiclient.Client
	ProjectID string
	SessionID string
	Threshold time.Duration
}

const queryKeyframe = `query($s: ID!){ memoryKeyframe(streamId: $s){ createdAt } }`

// StreamID is the session's keyframe stream.
func (c Config) StreamID() string { return session.StreamID(c.ProjectID, c.SessionID) }

// Run reads the session keyframe stream's latest keyframe via memoryKeyframe and returns the reminder
// text when there is no keyframe or it is older than Threshold, "" when it is fresh. When nothing
// answers it returns the transport error (apiclient.IsUnreachable) for the caller to report; any other
// failure (unauthorized, a GraphQL error) is silent, so a prompt hook never blocks or noises a session.
func Run(cfg Config) (string, error) {
	var out struct {
		MemoryKeyframe *struct {
			CreatedAt string `json:"createdAt"`
		} `json:"memoryKeyframe"`
	}
	if err := cfg.GQL.Query(queryKeyframe, map[string]any{"s": cfg.StreamID()}, &out); err != nil {
		if apiclient.IsUnreachable(err) {
			return "", err
		}
		return "", nil
	}
	if out.MemoryKeyframe == nil {
		return injectionText, nil
	}
	createdAt, err := time.Parse(time.RFC3339, out.MemoryKeyframe.CreatedAt)
	if err != nil {
		return injectionText, nil
	}
	if time.Since(createdAt) > cfg.Threshold {
		return injectionText, nil
	}
	return "", nil
}
