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
	"goalie/internal/motd"
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
	return "", config.ErrNotInitialised
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

func Log(ctx AppContext, note, goalID string, blocked, done bool, task string) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	if note == "" {
		var err error
		note, goalID, task, blocked, done, err = InteractiveLog(&ctx)
		if err != nil {
			return err
		}
	}
	if task == "" {
		fmt.Fprintln(ctx.Stderr, "Task tag is required — use --task #impl or pick one interactively")
		return &ExitError{Code: 1}
	}
	if !goals.ValidTaskTag(task) {
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
		Goal:          goalPtr,
		Note:          note,
		Blocked:       blocked,
		Done:          done,
		Task:          taskPtr,
		SchemaVersion: ctx.SchemaVersion,
	}, ctx.EncryptionKey)
}

// unblockLookupDays bounds how far back Unblock searches for the target's
// latest entry. Wider than the status/summary default window since a
// blocker can sit untouched for a while before someone notices it's cleared.
const unblockLookupDays = 30

// targetLabel formats a goal+task pair for user-facing messages, e.g.
// "ROUTING#impl" or just "#impl" when there is no goal.
func targetLabel(goal, task string) string {
	return goal + task
}

// Unblock records a new entry, as the current user, marking another user's
// blocked entry for the same (goal, task) as unblocked. It does not mutate
// the target entry — display code renders the target's [BLOCKED] tag as
// [UNBLOCKED] once an entry referencing it exists (see journal.CollectLatestAndUnblocked).
func Unblock(ctx AppContext, targetUsername, goalID, task, note string) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	if targetUsername == "" {
		fmt.Fprintln(ctx.Stderr, "Username is required — e.g. goalie unblock @alice --task #impl")
		return &ExitError{Code: 1}
	}
	if task == "" {
		fmt.Fprintln(ctx.Stderr, "Task tag is required — use --task #impl")
		return &ExitError{Code: 1}
	}
	if !goals.ValidTaskTag(task) {
		fmt.Fprintf(ctx.Stderr, "Task tag '%s' is invalid — use #lowercase, e.g. #impl\n", task)
		return &ExitError{Code: 1}
	}
	if goalID != "" && !goals.Exists(ctx.DataDir, goalID, ctx.EncryptionKey) {
		fmt.Fprintf(ctx.Stderr, "Goal '%s' does not exist\n", goalID)
		return &ExitError{Code: 1}
	}

	entries, unblockedTargets, err := journal.CollectLatestAndUnblocked(ctx.DataDir, ctx.Git, unblockLookupDays, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	target := journal.UnblockTarget{Username: targetUsername, Goal: goalID, Task: task}
	var found journal.Entry
	targetFound := false
	for _, e := range entries {
		if journal.TargetOf(e) == target {
			found = e
			targetFound = true
			break
		}
	}
	if !targetFound {
		fmt.Fprintf(ctx.Stderr, "No entry found for %s on %s\n", targetUsername, targetLabel(goalID, task))
		return &ExitError{Code: 1}
	}
	if !found.Blocked || journal.IsUnblocked(found, unblockedTargets) {
		fmt.Fprintf(ctx.Stderr, "The latest entry for %s on %s is not blocked\n", targetUsername, targetLabel(goalID, task))
		return &ExitError{Code: 1}
	}

	username, err := resolveUsername(ctx)
	if err != nil {
		return err
	}
	if note == "" {
		note = "unblocked"
	}
	return journal.Append(ctx.DataDir, ctx.Git, username, journal.Entry{
		Goal:          found.Goal,
		Task:          found.Task,
		Note:          note,
		Unblocks:      &targetUsername,
		SchemaVersion: ctx.SchemaVersion,
	}, ctx.EncryptionKey)
}

func Status(ctx AppContext, days int) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	if days <= 0 {
		days = ctx.EffectiveStatusDays()
	}
	if motdText, ok, err := motd.Latest(ctx.DataDir, ctx.EncryptionKey); err == nil && ok {
		fmt.Fprintln(ctx.Stdout, display.FormatMotd(motdText, ctx.DisplayCtx(), ctx.EffectiveWrapWidth()))
	}
	entries, unblockedTargets, err := journal.CollectLatestAndUnblocked(ctx.DataDir, ctx.Git, days, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintf(ctx.Stdout, "No entries in the last %d days.\n", days)
		return nil
	}

	now := time.Now()
	doneHideCutoff := journal.PriorBusinessDayStart(now)

	byUser := make(map[string][]journal.Entry)
	for _, e := range entries {
		if e.Done {
			ts, err := time.Parse(time.RFC3339, e.TS)
			if err != nil || ts.Before(doneHideCutoff) {
				continue
			}
		}
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

	selfUsername, _ := resolveUsername(ctx)

	const entryIndent = "  "
	availableWidth := ctx.EffectiveWrapWidth() - len(entryIndent)

	for _, u := range users {
		display.Section(u, ctx.Stdout, ctx.DisplayCtx())
		ues := byUser[u]
		journal.SortForDisplay(ues)
		for _, e := range ues {
			formatted := display.WrapStatusEntry(e, selfUsername, now, ctx.DisplayCtx(), availableWidth, unblockedTargets)
			for _, line := range strings.Split(formatted, "\n") {
				fmt.Fprintf(ctx.Stdout, "%s%s\n", entryIndent, line)
			}
		}
	}
	return nil
}


func MotdShow(ctx AppContext) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	text, ok, err := motd.Latest(ctx.DataDir, ctx.EncryptionKey)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(ctx.Stdout, "No MOTD set.")
		return nil
	}
	fmt.Fprintln(ctx.Stdout, text)
	return nil
}

func MotdSet(ctx AppContext, text string) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	return motd.Save(ctx.DataDir, ctx.Git, text, ctx.EncryptionKey)
}

func hasBlocked(entries []journal.Entry) bool {
	for _, e := range entries {
		if e.Blocked {
			return true
		}
	}
	return false
}

type summaryGroupKey struct {
	goal     string
	task     string
	username string
}

func Summary(ctx AppContext, days int, user string) error {
	if err := requireDataDir(ctx); err != nil {
		return err
	}
	selfUsername, _ := resolveUsername(ctx)
	var pattern string
	if user != "" {
		if strings.ContainsAny(user, "*?[") {
			pattern = user
		} else {
			if !strings.HasPrefix(user, "@") {
				user = "@" + user
			}
			pattern = user
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

	groups := make(map[summaryGroupKey][]journal.Entry)
	for _, e := range entries {
		task := ""
		if e.Task != nil {
			task = *e.Task
		}
		goal := "(no goal)"
		if e.Goal != nil {
			goal = *e.Goal
		}
		k := summaryGroupKey{goal: goal, task: task, username: e.Username}
		groups[k] = append(groups[k], e)
	}

	// Sort groups by most-recent entry timestamp, newest first.
	keys := make([]summaryGroupKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		latestI := groups[keys[i]][len(groups[keys[i]])-1].TS
		latestJ := groups[keys[j]][len(groups[keys[j]])-1].TS
		if latestI != latestJ {
			return latestI > latestJ
		}
		return keys[i].goal < keys[j].goal
	})

	now := time.Now()
	for gi, k := range keys {
		if gi > 0 {
			fmt.Fprintln(ctx.Stdout)
		}
		fmt.Fprintln(ctx.Stdout, display.FormatSummaryHeader(k.goal, k.task, k.username, ctx.DisplayCtx()))
		prevBlocked := false
		for _, e := range groups[k] {
			fmt.Fprintln(ctx.Stdout, display.FormatSummaryEntry(e, selfUsername, prevBlocked, now, ctx.DisplayCtx()))
			prevBlocked = e.Blocked
		}
	}
	return nil
}
