package cli

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"goalie/internal/config"
	"goalie/internal/display"
	"goalie/internal/goals"
	"goalie/internal/journal"
	"goalie/internal/slugify"
)

type blockedThread struct {
	tag   string
	state journal.ThreadState
}

type recentThread struct {
	tag   string
	state journal.ThreadState
}

func InteractiveUpdate(ctx *AppContext) error {
	var name, username string
	if ctx.Username != "" {
		name = ctx.Username
		username = ctx.Username
	} else {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		name = cfg.Name
		username = slugify.Slugify(cfg.Name)
	}

	r := bufio.NewReader(ctx.Stdin)

	fmt.Fprintf(ctx.Stdout, "Hi %s, let's review your threads.\n", name)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	states, err := journal.CurrentThreadStates(journalDir, username, ctx.EncryptionKey)
	if err != nil {
		return err
	}

	var blocked []blockedThread
	for tag, state := range states {
		if state.Blocked {
			blocked = append(blocked, blockedThread{tag: tag, state: state})
		}
	}

	if len(blocked) > 0 {
		fmt.Fprintf(ctx.Stdout, "%d blocked thread(s).\n", len(blocked))
	} else {
		fmt.Fprint(ctx.Stdout, "No blocked threads.\n")
	}

	display.Section("Blocked threads", ctx.Stdout, ctx.IsTTY)

	sort.Slice(blocked, func(i, j int) bool {
		gi, gj := "", ""
		if blocked[i].state.Goal != nil {
			gi = *blocked[i].state.Goal
		}
		if blocked[j].state.Goal != nil {
			gj = *blocked[j].state.Goal
		}
		if gi != gj {
			return gi < gj
		}
		return blocked[i].tag < blocked[j].tag
	})

	for _, item := range blocked {
		if item.state.Goal != nil {
			fmt.Fprintf(ctx.Stdout, "%s - %s - %s\n", *item.state.Goal, item.tag, item.state.Note)
		} else {
			fmt.Fprintf(ctx.Stdout, "%s - %s\n", item.tag, item.state.Note)
		}

		anyChanges, err := ynPrompt("Any changes or notes to record? (y/n) ", r, ctx.Stdout, ctx.IsTTY)
		if err != nil {
			return err
		}
		if !anyChanges {
			continue
		}

		unblocked, err := ynPrompt("Is it now unblocked? (y/n) ", r, ctx.Stdout, ctx.IsTTY)
		if err != nil {
			return err
		}

		fmt.Fprint(ctx.Stdout, display.Bold("Notes to add (enter to skip): ", ctx.IsTTY))
		noteLine, err := readLine(r)
		if err != nil {
			return err
		}
		note := strings.TrimSpace(noteLine)

		if note == "" && !unblocked {
			continue
		}

		entryNote := note
		if entryNote == "" {
			entryNote = "unblocked"
		}

		tag := item.tag
		if err := journal.Append(ctx.DataDir, ctx.Git, username, journal.Entry{
			Goal:    item.state.Goal,
			Note:    entryNote,
			Blocked: !unblocked,
			Thread:  &tag,
		}, ctx.EncryptionKey); err != nil {
			return err
		}
	}

	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	var recent []recentThread
	for tag, state := range states {
		if state.Blocked || state.TS == "" {
			continue
		}
		ts, parseErr := time.Parse(time.RFC3339, state.TS)
		if parseErr != nil {
			continue
		}
		if ts.Before(cutoff) {
			continue
		}
		recent = append(recent, recentThread{tag: tag, state: state})
	}

	sort.Slice(recent, func(i, j int) bool {
		return recent[i].state.TS > recent[j].state.TS
	})

	display.Section("Recent threads (last 7d)", ctx.Stdout, ctx.IsTTY)

	if len(recent) > 0 {
		fmt.Fprint(ctx.Stdout, "Your other recently active threads (last 7d):\n")
		for i, item := range recent {
			if item.state.Goal != nil {
				fmt.Fprintf(ctx.Stdout, "  %d. %s - %s - %s\n", i+1, *item.state.Goal, item.tag, item.state.Note)
			} else {
				fmt.Fprintf(ctx.Stdout, "  %d. %s - %s\n", i+1, item.tag, item.state.Note)
			}
		}

		doUpdate, err := ynPrompt("Do you want to update any of these? (y/n) ", r, ctx.Stdout, ctx.IsTTY)
		if err != nil {
			return err
		}
		if doUpdate {
			for {
				fmt.Fprint(ctx.Stdout, display.Bold("Enter number (or blank to finish): ", ctx.IsTTY))
				choice, err := readLine(r)
				if err != nil {
					return err
				}
				choice = strings.TrimSpace(choice)
				if choice == "" {
					break
				}
				n, parseErr := strconv.Atoi(choice)
				if parseErr != nil || n < 1 || n > len(recent) {
					fmt.Fprintf(ctx.Stdout, "Enter a number between 1 and %d, or blank to finish.\n", len(recent))
					continue
				}
				item := recent[n-1]

				isBlocked, err := ynPrompt("Are you blocked? (y/n) ", r, ctx.Stdout, ctx.IsTTY)
				if err != nil {
					return err
				}
				notePrompt := "Notes: "
				if isBlocked {
					notePrompt = "Notes (what is blocking you?): "
				}
				note, err := requireInput(notePrompt, r, ctx.Stdout, ctx.IsTTY)
				if err != nil {
					return err
				}
				tag := item.tag
				if err := journal.Append(ctx.DataDir, ctx.Git, username, journal.Entry{
					Goal:    item.state.Goal,
					Note:    note,
					Blocked: isBlocked,
					Thread:  &tag,
				}, ctx.EncryptionKey); err != nil {
					return err
				}
			}
		}
	}

	display.Section("New threads", ctx.Stdout, ctx.IsTTY)

	wantNew, err := ynPrompt("Have you started any new threads you want to log? (y/n) ", r, ctx.Stdout, ctx.IsTTY)
	if err != nil {
		return err
	}
	if !wantNew {
		return nil
	}

	for {
		goalID, err := SelectGoal(ctx.DataDir, ctx.EncryptionKey, r, ctx.Stdout, ctx.IsTTY)
		if err != nil {
			return err
		}

		var existing []string
		if goalID != "" {
			existing, err = journal.CollectThreads(ctx.DataDir, goalID, ctx.EncryptionKey)
			if err != nil {
				return err
			}
		}

		var tag string
		for {
			if len(existing) > 0 {
				for i, t := range existing {
					fmt.Fprintf(ctx.Stdout, "  %d. %s\n", i+1, t)
				}
				fmt.Fprint(ctx.Stdout, display.Bold("Thread? (number to continue, new #hashtag, or blank to skip) ", ctx.IsTTY))
			} else {
				fmt.Fprint(ctx.Stdout, display.Bold("Thread? (#hashtag or blank to skip) ", ctx.IsTTY))
			}

			answer, err := readLine(r)
			if err != nil {
				return err
			}
			answer = strings.TrimSpace(answer)

			if answer == "" {
				break
			}
			if len(existing) > 0 {
				n, numErr := strconv.Atoi(answer)
				if numErr == nil && n >= 1 && n <= len(existing) {
					tag = existing[n-1]
					break
				}
			}
			if goals.ValidThreadTag(answer) {
				tag = answer
				break
			}
			fmt.Fprint(ctx.Stdout, "Enter a number, a #hashtag, or leave blank.\n")
		}

		if tag != "" {
			isBlocked, err := ynPrompt("Are you blocked? (y/n) ", r, ctx.Stdout, ctx.IsTTY)
			if err != nil {
				return err
			}
			notePrompt := "Notes: "
			if isBlocked {
				notePrompt = "Notes (what is blocking you?): "
			}
			note, err := requireInput(notePrompt, r, ctx.Stdout, ctx.IsTTY)
			if err != nil {
				return err
			}
			var goalPtr *string
			if goalID != "" {
				goalPtr = &goalID
			}
			if err := journal.Append(ctx.DataDir, ctx.Git, username, journal.Entry{
				Goal:    goalPtr,
				Note:    note,
				Blocked: isBlocked,
				Thread:  &tag,
			}, ctx.EncryptionKey); err != nil {
				return err
			}
		}

		more, err := ynPrompt("Log another new thread? (y/n) ", r, ctx.Stdout, ctx.IsTTY)
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}

	return nil
}
