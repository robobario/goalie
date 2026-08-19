package exportfmt_test

import (
	"encoding/json"
	"testing"

	"goalie/internal/exportfmt"
)

func TestSchemaJSONIsValid(t *testing.T) {
	var v any
	if err := json.Unmarshal(exportfmt.SchemaJSON, &v); err != nil {
		t.Fatalf("schema.json is not valid JSON: %v", err)
	}
}

func TestEventRoundTrip(t *testing.T) {
	goal := "ROUTING"
	task := "#impl"
	unblocks := "bob"

	cases := []struct {
		name string
		v    any
	}{
		{
			name: "create_goal",
			v: exportfmt.CreateGoalEvent{
				Type:        exportfmt.TypeCreateGoal,
				ID:          "ROUTING",
				Description: "Implement the routing layer",
				Timestamp:   "2024-01-08T10:00:00Z",
			},
		},
		{
			name: "close_goal",
			v: exportfmt.CloseGoalEvent{
				Type: exportfmt.TypeCloseGoal,
				ID:   "ROUTING",
			},
		},
		{
			name: "log_entry",
			v: exportfmt.LogEntryEvent{
				Type:          exportfmt.TypeLogEntry,
				ID:            "550e8400-e29b-41d4-a716-446655440000",
				Timestamp:     "2024-01-08T11:00:00Z",
				Username:      "@alice",
				Goal:          &goal,
				Task:          &task,
				Blocked:       true,
				Done:          false,
				Note:          "blocked on review",
				SchemaVersion: "1.1.0",
				Unblocks:      &unblocks,
			},
		},
		{
			name: "log_entry nil goal and task",
			v: exportfmt.LogEntryEvent{
				Type:          exportfmt.TypeLogEntry,
				ID:            "550e8400-e29b-41d4-a716-446655440001",
				Timestamp:     "2024-01-08T11:00:00Z",
				Username:      "@bob",
				Goal:          nil,
				Task:          nil,
				Blocked:       false,
				Done:          true,
				Note:          "done",
				SchemaVersion: "1.1.0",
			},
		},
		{
			name: "set_motd",
			v: exportfmt.SetMotdEvent{
				Type:      exportfmt.TypeSetMotd,
				Timestamp: "2024-01-08T10:00:00Z",
				Content:   "hello team",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var typed exportfmt.TypedEvent
			if err := json.Unmarshal(b, &typed); err != nil {
				t.Fatalf("unmarshal TypedEvent: %v", err)
			}
			if typed.Type == "" {
				t.Error("type field missing after round-trip")
			}
		})
	}
}
