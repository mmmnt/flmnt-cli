package gate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const injectionText = `[QUORUM REMINDER] No recent keyframe found for this project stream.
Before starting your next task, call write_keyframe on the domain stream to checkpoint your current understanding.
Use list_streams to find the stream_id if needed.`

type Config struct {
	CoreURL   string
	ProjectID string
	Threshold time.Duration
}

type keyframeResponse struct {
	CreatedAt string `json:"created_at"`
}

var client = &http.Client{Timeout: 5 * time.Second}

func Run(cfg Config) (string, error) {
	url := fmt.Sprintf("%s/streams/%s::domain/keyframes/latest", cfg.CoreURL, cfg.ProjectID)
	resp, err := client.Get(url)
	if err != nil {
		return "", nil // Core unreachable — silent exit
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return injectionText, nil
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

	var kf keyframeResponse
	if err := json.NewDecoder(resp.Body).Decode(&kf); err != nil {
		return "", nil
	}

	createdAt, err := time.Parse(time.RFC3339, kf.CreatedAt)
	if err != nil {
		return injectionText, nil
	}

	if time.Since(createdAt) > cfg.Threshold {
		return injectionText, nil
	}

	return "", nil
}
