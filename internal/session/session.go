// Package session is the agent session's keyframe stream: the id it has, the stream made sure to exist
// when the session starts, and the line that tells the agent which stream_id to use. The session id is
// the agent harness's own, read from its hook input, so nothing depends on instructions an agent may
// ignore (ruling ae750072).
package session

import (
	"fmt"

	"github.com/mmmnt/flmnt-cli/internal/apiclient"
)

// StreamID is the keyframe stream of session in project.
func StreamID(project, session string) string { return project + "::" + session + "::keyframe" }

const queryStream = `query($streamId: ID!, $projectId: ID!){ memoryStream(streamId: $streamId, projectId: $projectId){ streamId } }`

const mutationCreate = `mutation($projectId: ID!, $type: String!, $name: String!, $sessionId: ID){ createMemoryStream(projectId: $projectId, type: $type, name: $name, sessionId: $sessionId){ streamId } }`

// Ensure makes sure the session's keyframe stream exists in project, creating it (stream type keyframe,
// topology session_id) when it is absent, and returns its id. An error is returned as the server or
// the transport gave it; apiclient.IsUnreachable tells the caller nothing answered.
func Ensure(gql *apiclient.Client, project, session string) (string, error) {
	id := StreamID(project, session)
	var read struct {
		MemoryStream *struct {
			StreamID string `json:"streamId"`
		} `json:"memoryStream"`
	}
	if err := gql.Query(queryStream, map[string]any{"streamId": id, "projectId": project}, &read); err != nil {
		return "", err
	}
	if read.MemoryStream != nil {
		return id, nil
	}
	var created struct {
		CreateMemoryStream struct {
			StreamID string `json:"streamId"`
		} `json:"createMemoryStream"`
	}
	vars := map[string]any{"projectId": project, "type": "keyframe", "name": "session " + session, "sessionId": session}
	if err := gql.Query(mutationCreate, vars, &created); err != nil {
		return "", err
	}
	if created.CreateMemoryStream.StreamID != "" {
		return created.CreateMemoryStream.StreamID, nil
	}
	return id, nil
}

// Line is what the session-start hook prints for the agent's context.
func Line(streamID string) string {
	return fmt.Sprintf("[flmnt session] This session's keyframe stream is %q. Pass stream_id %q to write_keyframe and read_keyframe for the rest of this session.", streamID, streamID)
}
