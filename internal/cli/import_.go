package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"goalie/internal/exportfmt"
	"goalie/internal/goals"
	"goalie/internal/journal"
	"goalie/internal/motd"
)

// Import reads a JSONL export from stdin and replays each gesture event into
// the data repository. Unknown event types are silently skipped to preserve
// forward compatibility with exports produced by newer versions of goalie.
//
// The data repository must already be initialised (goalie init) before running
// import. Import fails fast: the first error stops processing.
func Import(ctx AppContext) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}

	scanner := bufio.NewScanner(ctx.Stdin)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var typed exportfmt.TypedEvent
		if err := json.Unmarshal(line, &typed); err != nil {
			return fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}

		switch typed.Type {
		case exportfmt.TypeCreateGoal:
			var ev exportfmt.CreateGoalEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				return fmt.Errorf("line %d: malformed create_goal event: %w", lineNum, err)
			}
			if ev.ID == "" || ev.Created == "" {
				return fmt.Errorf("line %d: create_goal missing required fields", lineNum)
			}
			createdAt, err := time.Parse(time.RFC3339, ev.Created)
			if err != nil {
				return fmt.Errorf("line %d: create_goal invalid created timestamp %q: %w", lineNum, ev.Created, err)
			}
			if err := goals.AddAt(ctx.DataDir, ctx.Git, ev.ID, ev.Description, ctx.EncryptionKey, createdAt); err != nil {
				return fmt.Errorf("line %d: create_goal %q: %w", lineNum, ev.ID, err)
			}

		case exportfmt.TypeCloseGoal:
			var ev exportfmt.CloseGoalEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				return fmt.Errorf("line %d: malformed close_goal event: %w", lineNum, err)
			}
			if ev.ID == "" {
				return fmt.Errorf("line %d: close_goal missing id", lineNum)
			}
			if err := goals.Close(ctx.DataDir, ctx.Git, ev.ID, ctx.EncryptionKey); err != nil {
				return fmt.Errorf("line %d: close_goal %q: %w", lineNum, ev.ID, err)
			}

		case exportfmt.TypeLogEntry:
			var ev exportfmt.LogEntryEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				return fmt.Errorf("line %d: malformed log_entry event: %w", lineNum, err)
			}
			if ev.ID == "" || ev.TS == "" || ev.Username == "" {
				return fmt.Errorf("line %d: log_entry missing required fields", lineNum)
			}
			ts, err := time.Parse(time.RFC3339, ev.TS)
			if err != nil {
				return fmt.Errorf("line %d: log_entry invalid ts %q: %w", lineNum, ev.TS, err)
			}
			e := journal.Entry{
				ID:            ev.ID,
				Goal:          ev.Goal,
				Task:          ev.Task,
				Blocked:       ev.Blocked,
				Done:          ev.Done,
				Note:          ev.Note,
				SchemaVersion: ev.SchemaVersion,
				Unblocks:      ev.Unblocks,
			}
			if err := journal.AppendAt(ctx.DataDir, ctx.Git, ev.Username, e, ctx.EncryptionKey, ts); err != nil {
				return fmt.Errorf("line %d: log_entry: %w", lineNum, err)
			}

		case exportfmt.TypeSetMotd:
			var ev exportfmt.SetMotdEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				return fmt.Errorf("line %d: malformed set_motd event: %w", lineNum, err)
			}
			if ev.TS == "" || ev.Content == "" {
				return fmt.Errorf("line %d: set_motd missing required fields", lineNum)
			}
			ts, err := time.Parse(time.RFC3339, ev.TS)
			if err != nil {
				return fmt.Errorf("line %d: set_motd invalid ts %q: %w", lineNum, ev.TS, err)
			}
			if err := motd.SaveAt(ctx.DataDir, ctx.Git, ev.Content, ctx.EncryptionKey, ts); err != nil {
				return fmt.Errorf("line %d: set_motd: %w", lineNum, err)
			}

		default:
			// Unknown event type — skip for forward compatibility.
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	return nil
}
