package cmd

import "testing"

func TestClassifyHookMetric(t *testing.T) {
	cases := []struct {
		name     string
		payload  string
		wantOK   bool
		category string
		outcome  string
	}{
		{
			name:     "passing turbo run",
			payload:  `{"tool_input":{"command":"pnpm turbo test"},"tool_response":"all tests passed"}`,
			wantOK:   true,
			category: "turbo",
			outcome:  "ok",
		},
		{
			name:     "failing vitest",
			payload:  `{"tool_input":{"command":"vitest run"},"tool_response":"1 failed, 2 passed"}`,
			wantOK:   true,
			category: "vitest",
			outcome:  "fail",
		},
		{
			name:     "deploy classified as deploy",
			payload:  `{"tool_input":{"command":"cdk deploy McpStack"},"tool_response":""}`,
			wantOK:   true,
			category: "deploy",
			outcome:  "ok",
		},
		{
			name:     "error in response marks fail",
			payload:  `{"tool_input":{"command":"go test ./..."},"tool_response":"FAIL: error in pkg"}`,
			wantOK:   true,
			category: "go",
			outcome:  "fail",
		},
		{
			name:    "non-CI command ignored",
			payload: `{"tool_input":{"command":"ls -la"},"tool_response":"file.txt"}`,
			wantOK:  false,
		},
		{
			name:    "malformed json ignored",
			payload: `not json at all`,
			wantOK:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			labels, ok := classifyHookMetric([]byte(tc.payload))
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if labels["category"] != tc.category {
				t.Errorf("category = %q, want %q", labels["category"], tc.category)
			}
			if labels["outcome"] != tc.outcome {
				t.Errorf("outcome = %q, want %q", labels["outcome"], tc.outcome)
			}
			if labels["tool"] != "Bash" {
				t.Errorf("tool = %q, want Bash", labels["tool"])
			}
			if labels["command"] == "" {
				t.Errorf("command label empty")
			}
		})
	}
}
