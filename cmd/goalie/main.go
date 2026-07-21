package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"goalie/internal/cli"
	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/tui"
)

func requireKey(keyErr error, fn func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if keyErr != nil {
			fmt.Fprintln(os.Stderr, "No encryption key found. Run 'goalie key init' or 'goalie key import <hex-key>'.")
			return &cli.ExitError{Code: 1}
		}
		return fn(cmd, args)
	}
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx := cli.AppContext{
		DataDir: home + "/.goalie/data",
		Git:     &git.RealRunner{},
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		IsTTY:   term.IsTerminal(int(os.Stdout.Fd())),
	}

	key, keyErr := crypto.LoadKey()
	ctx.EncryptionKey = key

	var logGoal string
	var logBlocked bool
	var logThread string
	var summaryDays int
	var summaryUser string

	root := &cobra.Command{
		Use:           "goalie",
		Short:         "Team goal and blocker tracker",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if ctx.EncryptionKey == nil {
				fmt.Fprintln(os.Stderr, "No encryption key found. Generate one with: goalie key init")
				fmt.Fprintln(os.Stderr, "Or import an existing key with: goalie key import <hex-key>")
				os.Exit(1)
			}
			return tui.Run(&ctx)
		},
	}

	configPath := filepath.Join(home, ".goalie", "config.json")

	initCmd := &cobra.Command{
		Use:   "init <repo-url>",
		Short: "Clone or create the data branch in ~/.goalie/data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.Init(args[0], ctx.DataDir, configPath, ctx.Git, ctx.Stdin, ctx.Stdout, ctx.IsTTY)
		},
	}

	logCmd := &cobra.Command{
		Use:   "log [note]",
		Short: "Append a journal entry; interactive if note is omitted",
		Args:  cobra.MaximumNArgs(1),
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			note := ""
			if len(args) > 0 {
				note = args[0]
			}
			return cli.Log(ctx, note, logGoal, logBlocked, logThread)
		}),
	}
	logCmd.Flags().StringVar(&logGoal, "goal", "", "Goal ID to associate with this entry")
	logCmd.Flags().BoolVar(&logBlocked, "blocked", false, "Mark this entry as blocked")
	logCmd.Flags().StringVar(&logThread, "thread", "", "Thread tag to associate with this entry")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Morning standup view: latest entry per user×goal×thread, last 7 days",
		Args:  cobra.NoArgs,
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.Status(ctx)
		}),
	}

	summaryCmd := &cobra.Command{
		Use:   "summary",
		Short: "Your entries for the last N days (default 7); --user '*' for everyone",
		Args:  cobra.NoArgs,
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.Summary(ctx, summaryDays, summaryUser)
		}),
	}
	summaryCmd.Flags().IntVar(&summaryDays, "days", 7, "Number of days to include")
	summaryCmd.Flags().StringVar(&summaryUser, "user", "", "Filter by user name or glob")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Interactive end-of-day review: update threads, log new activity",
		Args:  cobra.NoArgs,
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			if err := ctx.Git.Run([]string{"pull"}, ctx.DataDir); err != nil {
				return err
			}
			return cli.InteractiveUpdate(&ctx)
		}),
	}

	goalCmd := &cobra.Command{
		Use:   "goal",
		Short: "Manage goals",
	}

	goalAddCmd := &cobra.Command{
		Use:   "add <ID> <DESCRIPTION>",
		Short: "Create a new open goal",
		Args:  cobra.ExactArgs(2),
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.GoalAdd(ctx, args[0], args[1])
		}),
	}

	goalCloseCmd := &cobra.Command{
		Use:   "close <ID>",
		Short: "Mark a goal as closed",
		Args:  cobra.ExactArgs(1),
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.GoalClose(ctx, args[0])
		}),
	}

	goalListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all goals with their state",
		Args:  cobra.NoArgs,
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.GoalList(ctx)
		}),
	}

	goalCmd.AddCommand(goalAddCmd, goalCloseCmd, goalListCmd)

	keyCmd := &cobra.Command{
		Use:   "key",
		Short: "Manage encryption key",
	}

	keyInitCmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a new encryption key and print the hex to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.KeyInit(ctx)
		},
	}

	keyImportCmd := &cobra.Command{
		Use:   "import <hex>",
		Short: "Import an existing 64-char hex encryption key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.KeyImport(ctx, args[0])
		},
	}

	keyCmd.AddCommand(keyInitCmd, keyImportCmd)
	root.AddCommand(initCmd, logCmd, statusCmd, summaryCmd, updateCmd, goalCmd, keyCmd)

	if err := root.Execute(); err != nil {
		var exitErr *cli.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
