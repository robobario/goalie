package exportfmt

// EventType constants for the JSONL export format.
const (
	TypeCreateGoal = "create_goal"
	TypeCloseGoal  = "close_goal"
	TypeLogEntry   = "log_entry"
	TypeSetMotd    = "set_motd"
)

// TypedEvent is a minimal struct used to read the "type" discriminator before
// unmarshaling into the concrete event type.
type TypedEvent struct {
	Type string `json:"type"`
}

// CreateGoalEvent records the user gesture of creating a goal.
type CreateGoalEvent struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Timestamp   string `json:"timestamp"`
}

// CloseGoalEvent records the user gesture of closing a goal.
type CloseGoalEvent struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// LogEntryEvent records the user gesture of logging a journal entry.
type LogEntryEvent struct {
	Type          string  `json:"type"`
	ID            string  `json:"id"`
	Timestamp     string  `json:"timestamp"`
	Username      string  `json:"username"`
	Goal          *string `json:"goal"`
	Task          *string `json:"task"`
	Blocked       bool    `json:"blocked"`
	Done          bool    `json:"done"`
	Note          string  `json:"note"`
	SchemaVersion string  `json:"schema_version"`
	Unblocks      *string `json:"unblocks,omitempty"`
}

// SetMotdEvent records the user gesture of publishing a message of the day.
type SetMotdEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
}
