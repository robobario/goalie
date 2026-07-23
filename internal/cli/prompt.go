package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"goalie/internal/display"
	"goalie/internal/goals"
	"goalie/internal/journal"
)

// readLine reads one line from r (strips trailing newline).
// If r is a *bufio.Reader, bufio.NewReader reuses it directly.
func readLine(r io.Reader) (string, error) {
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// ynPrompt reads lines from r until one is "y", "yes", "n", or "no" (case-insensitive).
// It writes the prompt to w using display.Bold if tty.
func ynPrompt(prompt string, r io.Reader, w io.Writer, tty bool) (bool, error) {
	reader := bufio.NewReader(r)
	for {
		fmt.Fprint(w, display.Bold(prompt, tty))
		line, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
	}
}

// requireInput reads lines from r until a non-empty line is given.
func requireInput(prompt string, r io.Reader, w io.Writer, tty bool) (string, error) {
	reader := bufio.NewReader(r)
	for {
		fmt.Fprint(w, display.Bold(prompt, tty))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		if value := strings.TrimSpace(line); value != "" {
			return value, nil
		}
	}
}

// SelectGoal presents open goals from dataDir and returns the chosen goal ID,
// or "" if the user skips. Returns error on I/O failure.
func SelectGoal(dataDir string, key []byte, r io.Reader, w io.Writer, tty bool) (string, error) {
	all, err := goals.List(dataDir, key)
	if err != nil {
		return "", err
	}
	var open []goals.Goal
	for _, g := range all {
		if g.State == "open" {
			open = append(open, g)
		}
	}
	if len(open) == 0 {
		fmt.Fprint(w, "To create a new goal, use: goalie goal add <ID> <DESCRIPTION>\n")
		return "", nil
	}
	fmt.Fprint(w, "Goals:\n")
	for i, g := range open {
		label := g.ID
		if g.Description != "" {
			label = g.ID + " — " + g.Description
		}
		fmt.Fprintf(w, "  %d. %s\n", i+1, label)
	}
	reader := bufio.NewReader(r)
	for {
		fmt.Fprint(w, display.Bold("Which goal? (number, or blank to skip) ", tty))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		choice := strings.TrimSpace(line)
		if choice == "" {
			return "", nil
		}
		n, parseErr := strconv.Atoi(choice)
		if parseErr == nil && n >= 1 && n <= len(open) {
			return open[n-1].ID, nil
		}
		fmt.Fprintf(w, "Enter a number between 1 and %d, or blank to skip.\n", len(open))
	}
}

// InteractiveLog ports _interactive_log: prompts for goal, task, blocked, note.
func InteractiveLog(ctx *AppContext) (note, goalID, task string, blocked bool, err error) {
	r := bufio.NewReader(ctx.Stdin)

	goalID, err = SelectGoal(ctx.DataDir, ctx.EncryptionKey, r, ctx.Stdout, ctx.IsTTY)
	if err != nil {
		return
	}

	var existing []string
	if goalID != "" {
		existing, err = journal.CollectTasks(ctx.DataDir, goalID, ctx.EncryptionKey)
		if err != nil {
			return
		}
	}
	for {
		if len(existing) > 0 {
			for i, t := range existing {
				fmt.Fprintf(ctx.Stdout, "  %d. %s\n", i+1, t)
			}
			fmt.Fprint(ctx.Stdout, display.Bold("Task? (number or new #hashtag): ", ctx.IsTTY))
		} else {
			fmt.Fprint(ctx.Stdout, display.Bold("Task? (#hashtag): ", ctx.IsTTY))
		}
		var line string
		line, err = readLine(r)
		if err != nil {
			return
		}
		answer := strings.TrimSpace(line)
		if len(existing) > 0 {
			n, numErr := strconv.Atoi(answer)
			if numErr == nil && n >= 1 && n <= len(existing) {
				task = existing[n-1]
				break
			}
		}
		if goals.ValidTaskTag(answer) {
			task = answer
			break
		}
		fmt.Fprint(ctx.Stdout, "Enter a number or a #hashtag.\n")
	}

	blocked, err = ynPrompt("Are you blocked? (y/n) ", r, ctx.Stdout, ctx.IsTTY)
	if err != nil {
		return
	}

	notePrompt := "Notes: "
	if blocked {
		notePrompt = "Notes (what is blocking you?): "
	}
	note, err = requireInput(notePrompt, r, ctx.Stdout, ctx.IsTTY)
	return
}
