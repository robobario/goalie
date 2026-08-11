package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"goalie/internal/cli"
	"goalie/internal/config"
	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/goalieenv"
	"goalie/internal/meta"
	"goalie/internal/schema"
	"goalie/internal/state"
	"goalie/internal/tui"
	semver "goalie/internal/version"
	"goalie/internal/versiontrack"
)

var version = "dev"

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
	goalieHome, err := goalieenv.Home()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	dataDir := filepath.Join(goalieHome, "data")
	ctx := cli.AppContext{
		DataDir:       dataDir,
		Git:           &schemaGuardRunner{inner: &git.RealRunner{}, dataDir: dataDir},
		Stdin:         os.Stdin,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		IsTTY:         term.IsTerminal(int(os.Stdout.Fd())),
		SchemaVersion: schema.Version,
	}

	key, keyErr := crypto.LoadKey()
	ctx.EncryptionKey = key

	// If the data dir exists and meta says plaintext mode, no key is required.
	if _, statErr := os.Stat(ctx.DataDir); statErr == nil {
		m, metaErr := meta.Load(ctx.DataDir)
		if metaErr != nil {
			fmt.Fprintln(os.Stderr, metaErr)
			os.Exit(1)
		}
		if !m.Encrypt {
			keyErr = nil
			ctx.EncryptionKey = nil
		}
		if err := enforceSchemaCompatibility(ctx.DataDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		checkVersionOnce(ctx.DataDir, ctx.Git, os.Stderr)
	}

	var plainOutput bool
	var logGoal string
	var logBlocked bool
	var logDone bool
	var logTask string
	var statusDays int
	var summaryDays int
	var summaryUser string

	root := &cobra.Command{
		Use:           "goalie",
		Short:         "Team goal and blocker tracker",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyErr != nil {
				fmt.Fprintln(os.Stderr, "No encryption key found. Generate one with: goalie key init")
				fmt.Fprintln(os.Stderr, "Or import an existing key with: goalie key import <hex-key>")
				os.Exit(1)
			}
			return tui.Run(&ctx)
		},
	}

	root.PersistentFlags().BoolVar(&plainOutput, "plain", false, "Disable colour output")
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if plainOutput {
			ctx.IsTTY = false
		}
		cfg, err := config.Load()
		if err == nil && cfg != nil {
			ctx.Config = cfg
			ctx.WrapWidth = cfg.EffectiveWrapWidth()
			ctx.HyperLinks = ctx.IsTTY && cfg.EffectiveCompressHyperLinks()
			ctx.StatusDays = cfg.EffectiveStatusDays()
			if ctx.Username == "" {
				ctx.Username = cfg.Name
			}
		} else {
			ctx.WrapWidth = config.DefaultWrapWidth
			ctx.StatusDays = config.DefaultStatusDays
		}
	}

	configPath := filepath.Join(goalieHome, "config.json")

	var initBranch string
	initCmd := &cobra.Command{
		Use:   "init <repo-url>",
		Short: "Clone or create the data branch in ~/.goalie/data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.Init(args[0], ctx.DataDir, configPath, initBranch, ctx.Git, ctx.Stdin, ctx.Stdout, ctx.IsTTY)
		},
	}
	initCmd.Flags().StringVar(&initBranch, "branch", "data", "Git branch name to use for the data branch")

	logCmd := &cobra.Command{
		Use:   "log [note]",
		Short: "Append a journal entry; interactive if note is omitted",
		Args:  cobra.MaximumNArgs(1),
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			note := ""
			if len(args) > 0 {
				note = args[0]
			}
			return cli.Log(ctx, note, logGoal, logBlocked, logDone, logTask)
		}),
	}
	logCmd.Flags().StringVar(&logGoal, "goal", "", "Goal ID to associate with this entry")
	logCmd.Flags().BoolVar(&logBlocked, "blocked", false, "Mark this entry as blocked")
	logCmd.Flags().BoolVar(&logDone, "done", false, "Mark the task as done")
	logCmd.Flags().StringVar(&logTask, "task", "", "Task tag to associate with this entry")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Morning standup view: latest entry per user×goal×task (default last 8 days)",
		Args:  cobra.NoArgs,
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.Status(ctx, statusDays)
		}),
	}
	statusCmd.Flags().IntVar(&statusDays, "days", 0, "Number of days to include (0 = use config, default 8)")

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
		Short: "Interactive end-of-day review: update tasks, log new activity",
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

	motdCmd := &cobra.Command{
		Use:   "motd",
		Short: "Show the team message of the day",
		Args:  cobra.NoArgs,
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.MotdShow(ctx)
		}),
	}

	motdSetCmd := &cobra.Command{
		Use:   "set <text>",
		Short: "Publish a new message of the day",
		Args:  cobra.ExactArgs(1),
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.MotdSet(ctx, args[0])
		}),
	}

	motdCmd.AddCommand(motdSetCmd)

	skillsCmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage Claude Code skills",
	}

	skillsInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Install Claude Code skills to ~/.claude/skills/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.SkillsInstall(ctx)
		},
	}

	skillsUpdateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update installed Claude Code skills (overwrites existing)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.SkillsUpdate(ctx)
		},
	}

	skillsRemoveCmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove installed Claude Code skills from ~/.claude/skills/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.SkillsRemove(ctx)
		},
	}

	skillsCmd.AddCommand(skillsInstallCmd, skillsUpdateCmd, skillsRemoveCmd)

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Dump all data as JSONL for debugging and schema compatibility testing",
		Args:  cobra.NoArgs,
		RunE: requireKey(keyErr, func(cmd *cobra.Command, args []string) error {
			return cli.Export(ctx)
		}),
	}

	root.AddCommand(initCmd, logCmd, statusCmd, summaryCmd, updateCmd, goalCmd, keyCmd, motdCmd, skillsCmd, exportCmd)

	if err := root.Execute(); err != nil {
		var exitErr *cli.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// checkVersionOnce runs the version-tracking check at most once per 24 hours.
// Failures are silent — version tracking must never block normal usage.
func checkVersionOnce(dataDir string, r git.Runner, stderr interface{ Write([]byte) (int, error) }) {
	s, err := state.Load()
	if err != nil || !state.VersionCheckDue(s) {
		return
	}

	highest, err := versiontrack.Record(dataDir, r, schema.Version)
	if err != nil {
		return
	}

	s.LastVersionCheck = time.Now().UTC().Format(time.RFC3339)
	_ = state.Save(s)

	if semver.Compare(highest, schema.Version) <= 0 {
		return
	}

	fmt.Fprintf(stderr, "Note: team members are using schema version %s (you have %s). Consider upgrading goalie.\n", highest, schema.Version)
}

// enforceSchemaCompatibility reads the local versions/ directory and fails if
// the highest recorded schema major version exceeds the one this binary supports.
// It reads local state only — no git pull — so commands remain fast.
func enforceSchemaCompatibility(dataDir string) error {
	highest, err := versiontrack.HighestRecorded(dataDir)
	if err != nil || highest == "" {
		return nil
	}
	myMajor := semver.Major(schema.Version)
	theirMajor := semver.Major(highest)
	if myMajor >= 0 && theirMajor > myMajor {
		return fmt.Errorf("data repo uses schema version %s (major %d) but this binary only supports major %d — upgrade goalie before running any commands", highest, theirMajor, myMajor)
	}
	return nil
}

// schemaGuardRunner wraps a git.Runner and re-checks schema compatibility after
// every pull on the data directory, so a major version mismatch discovered via
// a pull causes an immediate error rather than silently completing.
type schemaGuardRunner struct {
	inner   git.Runner
	dataDir string
}

func (r *schemaGuardRunner) Run(args []string, cwd string) error {
	if err := r.inner.Run(args, cwd); err != nil {
		return err
	}
	if len(args) > 0 && args[0] == "pull" && cwd == r.dataDir {
		return enforceSchemaCompatibility(r.dataDir)
	}
	return nil
}

func (r *schemaGuardRunner) Output(args []string, cwd string) (string, error) {
	return r.inner.Output(args, cwd)
}
