package corpus

import "github.com/mmmnt/flmnt-cli/internal/apiclient"

// querySlice reads a whole stream. memorySlice with no bounds is the unbounded window, oldest-first,
// which is what a document render needs: the order the doctrine was authored in.
const querySlice = `query($s: ID!){ memorySlice(streamId: $s){ id causationId entryType timestamp content } }`

// Fetch reads every entry of a stream through the authenticated router GraphQL.
func Fetch(gql *apiclient.Client, streamID string) ([]Entry, error) {
	var out struct {
		MemorySlice []struct {
			ID          string `json:"id"`
			CausationID string `json:"causationId"`
			EntryType   string `json:"entryType"`
			Timestamp   string `json:"timestamp"`
			Content     string `json:"content"`
		} `json:"memorySlice"`
	}
	if err := gql.Query(querySlice, map[string]any{"s": streamID}, &out); err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(out.MemorySlice))
	for _, e := range out.MemorySlice {
		entries = append(entries, Entry{ID: e.ID, CausationID: e.CausationID, EntryType: e.EntryType, Timestamp: e.Timestamp, Content: e.Content})
	}
	return entries, nil
}
