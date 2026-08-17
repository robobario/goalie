package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"goalie/internal/goals"
	"goalie/internal/journal"
	"goalie/internal/motd"
)

// Export writes a deterministic JSONL snapshot of every data entity in the
// repository to stdout. One JSON object per line; ordering is stable so diffs
// are meaningful. Output is identical regardless of whether the repo is
// encrypted — successful output proves decryption worked.
//
// Intended for schema compatibility testing and debugging.
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

	// Goals — sorted ascending by ID.
	goalList, err := goals.List(ctx.DataDir, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	for _, g := range goalList {
		if err := emit(struct {
			Type        string `json:"type"`
			ID          string `json:"id"`
			State       string `json:"state"`
			Description string `json:"description"`
			Created     string `json:"created"`
		}{"goal", g.ID, g.State, g.Description, g.Created}); err != nil {
			return err
		}
	}

	// Entries — descending by ts, then ascending username / goal / task.
	entries, err := journal.ReadAll(ctx.DataDir, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.TS != b.TS {
			return a.TS > b.TS
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
		if err := emit(struct {
			Type          string  `json:"type"`
			ID            string  `json:"id"`
			TS            string  `json:"ts"`
			Username      string  `json:"username"`
			Goal          *string `json:"goal"`
			Task          *string `json:"task"`
			Blocked       bool    `json:"blocked"`
			Done          bool    `json:"done"`
			Note          string  `json:"note"`
			SchemaVersion string  `json:"schema_version"`
			Unblocks      *string `json:"unblocks,omitempty"`
		}{
			Type:          "entry",
			ID:            e.ID,
			TS:            e.TS,
			Username:      e.Username,
			Goal:          e.Goal,
			Task:          e.Task,
			Blocked:       e.Blocked,
			Done:          e.Done,
			Note:          e.Note,
			SchemaVersion: e.SchemaVersion,
			Unblocks:      e.Unblocks,
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
		if err := emit(struct {
			Type    string `json:"type"`
			TS      string `json:"ts"`
			Content string `json:"content"`
		}{"motd", m.TS, m.Content}); err != nil {
			return err
		}
	}

	// Version files — ascending by schema_version.
	versDir := filepath.Join(ctx.DataDir, "versions")
	versEntries, _ := os.ReadDir(versDir)
	var versionValues []string
	for _, ve := range versEntries {
		if ve.IsDir() || filepath.Ext(ve.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(versDir, ve.Name()))
		if err != nil {
			continue
		}
		var vf struct {
			SchemaVersion string `json:"schema_version"`
		}
		if json.Unmarshal(data, &vf) == nil && vf.SchemaVersion != "" {
			versionValues = append(versionValues, vf.SchemaVersion)
		}
	}
	sort.Strings(versionValues)
	for _, v := range versionValues {
		if err := emit(struct {
			Type          string `json:"type"`
			SchemaVersion string `json:"schema_version"`
		}{"version", v}); err != nil {
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
