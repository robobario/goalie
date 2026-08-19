package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"goalie/internal/exportfmt"
	"goalie/internal/goals"
	"goalie/internal/journal"
	"goalie/internal/motd"
)

// Export writes a deterministic JSONL snapshot of every data entity in the
// repository to stdout as typed gesture events. One JSON object per line;
// ordering is stable so diffs are meaningful. Output is identical regardless
// of whether the repo is encrypted — successful output proves decryption worked.
//
// Each line is validated against the exportfmt typed structs before emission.
// Unknown event types produced by newer versions of goalie are silently skipped
// on import, preserving forward compatibility.
func Export(ctx AppContext) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	if err := ctx.Git.Run([]string{"pull"}, ctx.DataDir); err != nil {
		return err
	}

	emit := func(v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(ctx.Stdout, string(b))
		return err
	}

	// Goals — sorted ascending by ID, then closed goals emit an additional event.
	goalList, err := goals.List(ctx.DataDir, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	for _, g := range goalList {
		if err := emit(exportfmt.CreateGoalEvent{
			Type:        exportfmt.TypeCreateGoal,
			ID:          g.ID,
			Description: g.Description,
			Timestamp:   g.Created,
		}); err != nil {
			return err
		}
		if g.State == "closed" {
			if err := emit(exportfmt.CloseGoalEvent{
				Type: exportfmt.TypeCloseGoal,
				ID:   g.ID,
			}); err != nil {
				return err
			}
		}
	}

	// Entries — sorted ascending by ts so an unblock replays after the entry it unblocks,
	// then ascending username / goal / task.
	entries, err := journal.ReadAll(ctx.DataDir, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.TS != b.TS {
			return a.TS < b.TS
		}
		if a.Username != b.Username {
			return a.Username < b.Username
		}
		ag, bg := ptrStr(a.Goal), ptrStr(b.Goal)
		if ag != bg {
			return ag < bg
		}
		return ptrStr(a.Task) < ptrStr(b.Task)
	})
	for _, e := range entries {
		if err := emit(exportfmt.LogEntryEvent{
			Type:      exportfmt.TypeLogEntry,
			ID:        e.ID,
			Timestamp: e.TS,
			Username:  e.Username,
			Goal:      e.Goal,
			Task:      e.Task,
			Blocked:   e.Blocked,
			Done:      e.Done,
			Note:      e.Note,
			Unblocks:  e.Unblocks,
		}); err != nil {
			return err
		}
	}

	// MOTDs — chronological order (ascending by filename timestamp).
	motdEntries, err := motd.AllEntries(ctx.DataDir, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	for _, m := range motdEntries {
		if err := emit(exportfmt.SetMotdEvent{
			Type:      exportfmt.TypeSetMotd,
			Timestamp: m.TS,
			Content:   m.Content,
		}); err != nil {
			return err
		}
	}

	return nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
