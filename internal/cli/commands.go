package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"goalie/internal/config"
	"goalie/internal/display"
	"goalie/internal/goals"
	"goalie/internal/journal"
	"goalie/internal/slugify"
)

func requireDataDir(ctx AppContext) error {
	if _, err := os.Stat(ctx.DataDir); os.IsNotExist(err) {
		fmt.Fprintln(ctx.Stderr, "Run 'goalie init <repo-url>' first.")
		return &ExitError{Code: 1}
	}
	return nil
}

func resolveUsername(ctx AppContext) (string, error) {
	if ctx.Username != "" {
		return ctx.Username, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	return slugify.Slugify(cfg.Name), nil
}

func GoalAdd(ctx AppContext, id, desc string) error {
	if !goals.ValidGoalID(id) {
		fmt.Fprintf(ctx.Stderr, "Goal ID '%s' is invalid — use uppercase letters, digits, and underscores, e.g. ROUTING_RUNTIME\n", id)
		return &ExitError{Code: 1}
	}
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	if err := goals.Add(ctx.DataDir, ctx.Git, id, desc, ctx.EncryptionKey); err != nil {
		if err == goals.ErrGoalExists {
			fmt.Fprintf(ctx.Stderr, "Goal '%s' already exists\n", id)
			return &ExitError{Code: 1}
		}
		return err
	}
	return nil
}

func GoalClose(ctx AppContext, id string) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	if err := goals.Close(ctx.DataDir, ctx.Git, id, ctx.EncryptionKey); err != nil {
		switch err {
		case goals.ErrGoalNotFound:
			fmt.Fprintf(ctx.Stderr, "Goal '%s' does not exist\n", id)
			return &ExitError{Code: 1}
		case goals.ErrGoalClosed:
			fmt.Fprintf(ctx.Stderr, "Goal '%s' is already closed\n", id)
			return &ExitError{Code: 1}
		}
		return err
	}
	return nil
}

func GoalList(ctx AppContext) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	list, err := goals.List(ctx.DataDir, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	for _, g := range list {
		fmt.Fprintf(ctx.Stdout, "%s\t%s\t%s\n", g.ID, g.State, g.Description)
	}
	return nil
}

func Log(ctx AppContext, note, goalID string, blocked bool, task string) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	if note == "" {
		var err error
		note, goalID, task, blocked, err = InteractiveLog(&ctx)
		if err != nil {
			return err
		}
	}
	if task != "" && !goals.ValidTaskTag(task) {
		fmt.Fprintf(ctx.Stderr, "Task tag '%s' is invalid — use #lowercase, e.g. #impl\n", task)
		return &ExitError{Code: 1}
	}
	if goalID != "" && !goals.Exists(ctx.DataDir, goalID, ctx.EncryptionKey) {
		fmt.Fprintf(ctx.Stderr, "Goal '%s' does not exist\n", goalID)
		return &ExitError{Code: 1}
	}
	username, err := resolveUsername(ctx)
	if err != nil {
		return err
	}
	var goalPtr *string
	if goalID != "" {
		goalPtr = &goalID
	}
	var taskPtr *string
	if task != "" {
		taskPtr = &task
	}
	return journal.Append(ctx.DataDir, ctx.Git, username, journal.Entry{
		Goal:    goalPtr,
		Note:    note,
		Blocked: blocked,
		Task:    taskPtr,
	}, ctx.EncryptionKey)
}

func Status(ctx AppContext) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	entries, err := journal.CollectLatest(ctx.DataDir, ctx.Git, 7, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprint(ctx.Stdout, "No entries in the last 7 days.\n")
		return nil
	}

	byUser := make(map[string][]journal.Entry)
	for _, e := range entries {
		byUser[e.Username] = append(byUser[e.Username], e)
	}

	users := make([]string, 0, len(byUser))
	for u := range byUser {
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool {
		bi := hasBlocked(byUser[users[i]])
		bj := hasBlocked(byUser[users[j]])
		if bi != bj {
			return bi
		}
		return users[i] < users[j]
	})

	now := time.Now().UTC()
	for _, u := range users {
		display.Section(u, ctx.Stdout, ctx.IsTTY)
		ues := byUser[u]
		sort.Slice(ues, func(i, j int) bool {
			if ues[i].Blocked != ues[j].Blocked {
				return ues[i].Blocked
			}
			return ues[i].TS < ues[j].TS
		})
		for _, e := range ues {
			fmt.Fprintf(ctx.Stdout, "  %s\n", display.FormatStatusEntry(e, now, ctx.IsTTY))
		}
	}
	return nil
}

func hasBlocked(entries []journal.Entry) bool {
	for _, e := range entries {
		if e.Blocked {
			return true
		}
	}
	return false
}

func Summary(ctx AppContext, days int, user string) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	var pattern string
	if user != "" {
		if strings.ContainsAny(user, "*?[") {
			pattern = user
		} else {
			pattern = slugify.Slugify(user)
		}
	} else {
		username, err := resolveUsername(ctx)
		if err != nil {
			return err
		}
		pattern = username
	}
	entries, err := journal.Collect(ctx.DataDir, ctx.Git, days, pattern, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintf(ctx.Stdout, "No entries in the last %d days.\n", days)
		return nil
	}
	now := time.Now().UTC()
	for _, e := range entries {
		fmt.Fprintln(ctx.Stdout, display.FormatEntry(e, now, ctx.IsTTY))
	}
	return nil
}
