package gate

import (
	"time"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

const injectionText = `[QUORUM REMINDER] No recent keyframe found for this project stream.
Before starting your next task, call write_keyframe on the domain stream to checkpoint your current understanding.
Use list_streams to find the stream_id if needed.`

// Config is the context-recency check. GQL is the authenticated router GraphQL client so the keyframe
// read is scoped to the caller's workspace; a stale/missing keyframe triggers the reminder.
type Config struct {
	GQL       *apiclient.Client
	ProjectID string
	Threshold time.Duration
}

const queryKeyframe = `query($s: ID!){ memoryKeyframe(streamId: $s){ createdAt } }`

// Run reads the domain stream's latest keyframe via memoryKeyframe and returns the reminder text when
// there is no keyframe or it is older than Threshold. Silent ("") when fresh or the router is unreachable.
func Run(cfg Config) (string, error) {
	var out struct {
		MemoryKeyframe *struct {
			CreatedAt string `json:"createdAt"`
		} `json:"memoryKeyframe"`
	}
	if err := cfg.GQL.Query(queryKeyframe, map[string]any{"s": cfg.ProjectID + "::domain"}, &out); err != nil {
		return "", nil // router unreachable / unauthorized — silent exit
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
