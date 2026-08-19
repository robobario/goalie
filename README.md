Goalie helps a remote team of open source developers share their goals and surface blockers.

## How it works

Add shared goals, then create tasks using hashtags. Log updates to show whether you are blocked or making progress. Run `goalie` with no arguments to open the TUI — a two-tab interface showing team activity and a guided end-of-day update flow.

Own your data. Your goals and updates are stored in the git server you point `goalie` at. Every team member writes updates to the shared git server, so no server to maintain, you can piggyback off your established Git authentication and authorization.

<img width="1200" height="675" alt="Image" src="https://github.com/user-attachments/assets/d87b1095-823e-4431-a3ca-3aa8e34d5a17" />

## Installation

**Prerequisites:** git.

Download the archive for your platform from the [releases page](../../releases) or build from source with `./build.sh` (requires Docker).

Extract and copy the binary to a directory on your `PATH`:

```sh
# Linux
tar -xzf goalie_*_linux_amd64.tar.gz
cp goalie ~/bin/goalie

# macOS (Apple Silicon)
tar -xzf goalie_*_darwin_arm64.tar.gz
cp goalie ~/bin/goalie

# macOS (Intel)
tar -xzf goalie_*_darwin_amd64.tar.gz
cp goalie ~/bin/goalie

chmod +x ~/bin/goalie
```

To keep goalie up to date, replace the binary with a newer build. Goalie warns once per day if a team member is running a newer data schema version, and refuses to start if a major schema version upgrade requires migration.

By default goalie stores all data under `~/.goalie`. Set the `GOALIE_HOME` environment variable to use a different directory.

## Team Setup

Goalie stores goals and journal entries in a dedicated `data` branch of a shared git repository. You need an existing repo that all team members can push to.

### First person on the team

Run `init`, pointing at the shared repo. Goalie creates the `data` branch, asks for your username, and asks whether to enable client-side encryption.

```sh
goalie init https://github.com/your-org/your-repo.git
```

The username prompt displays a fixed `@` prefix — type just the handle body (e.g. `alice` or `alice-jones`). Usernames follow GitHub handle rules: letters, digits, and hyphens, starting with a letter or digit. The `@` is stored as part of the username and shown in all output.

**Should you enable encryption?** The client-side encryption in goalie is minimal — it is intended for experimentation and light privacy, not as a security guarantee. For any real team data, use a private, access-controlled git repository as your primary protection. The encryption option exists for cases where the repo is public or semi-public and you want a basic layer of obscurity on top; it is not a substitute for proper access control.

If you enable encryption, `goalie init` generates a key, commits a `key-check.enc` sentinel to the data branch, and prints the key:

```
Encryption key: a1b2c3d4...
Share with teammates: goalie key import <key>
key-check.enc committed to the data branch — teammates must import the same key.
```

**Copy the hex key and share it securely with every teammate** (e.g. a password manager, a private message). Without it they cannot read or write data.

Once setup is done, create the team's first goals:

```sh
goalie goal add FEATURE_X "Implement feature X"
goalie goal add BUG_Y "Fix the production bug"
```

### Joining the team

Get the repo URL and, if encryption is enabled, the hex key from the person who ran the first `goalie init`.

```sh
goalie init https://github.com/your-org/your-repo.git
```

Goalie clones the `data` branch, asks for your username, and — if the repo uses encryption — immediately asks for the key:

```
Encryption key (paste hex or press Enter to skip): a1b2c3d4...
Encryption key verified.
```

Paste the hex key you received. Goalie verifies it against `key-check.enc` before saving. If you don't have the key yet, press Enter to skip and import it later:

```sh
goalie key import <hex-key>
```

### Using a different branch name

By default goalie uses a branch called `data`. To experiment without touching the team's real data, pass `--branch` to `goalie init`:

```sh
goalie init https://github.com/your-org/your-repo.git --branch data-test
```

All teammates who want to work against that branch must also pass `--branch data-test` when running `goalie init`.

### Replacing or rotating a key

Use `goalie key init` to generate a new key or `goalie key import <hex>` to import one. Both commands warn you before overwriting an existing key file, since replacing it will prevent you from decrypting data written under the old key.

## Daily Workflow

**Check what the team is working on:**

```sh
goalie status          # morning standup view — latest entry per person/task, last 7 days
```

**Log what you are doing:**

```sh
goalie log "started the API layer" --task #impl --goal FEATURE_X
goalie log "hit a dependency issue" --task #impl --goal FEATURE_X --blocked
goalie log "dependency resolved, back to it" --task #impl --goal FEATURE_X
goalie log "shipped" --task #impl --goal FEATURE_X --done
```

**Review your own history:**

```sh
goalie summary                        # your entries for the last 7 days
goalie summary --days 14              # last two weeks
goalie summary --user "*"             # everyone on the team
```

**Use the TUI** for end-of-day updates — run `goalie` with no arguments.

## Backup and migration

`goalie export` writes the whole data repo to stdout as JSONL — one gesture
event per line (goals created and closed, journal entries, MOTDs). `goalie
import` reads that stream from stdin and replays it into a data repo that has
already been set up with `goalie init`.

```sh
goalie export > backup.jsonl                              # snapshot the current data repo
goalie import < backup.jsonl                             # replay it into an initialised repo
goalie export | GOALIE_HOME=~/.goalie-copy goalie import  # pipe into a different repo
```

Export is encryption-agnostic: the output is always plaintext JSONL regardless
of whether the data repo is encrypted. Import writes using whatever encryption
the target repo was initialised with, so export/import doubles as the way to
move data between a plaintext and an encrypted repo, or to rotate to a new key.

Events are ordered so dependencies come before dependents — a goal is created
before any entry references it, and an entry appears before the later entry that
unblocks it — so import can replay the stream in a single top-to-bottom pass.
The format is forward-compatible: an older `goalie import` silently skips event
types it does not recognise, so a newer export still loads. See
[ARCHITECTURE.md](ARCHITECTURE.md) for the full format contract.

Import fails fast — the first malformed line or replay error stops the run.

## Usage

```
goalie                              # Open the TUI (activity view + guided update)
goalie init <repo-url> [--branch NAME]
                                    # Clone or create the data branch (default: "data"); prompts for name and encryption
goalie goal add <ID> <DESCRIPTION>  # Create a new open goal
goalie goal close <ID>              # Mark a goal as closed
goalie goal list                    # List all goals with their state
goalie log [note] [--goal ID] --task TAG [--blocked] [--done]
                                    # Append a journal entry; interactive if note is omitted. --task is required.
                                    # --done marks the task closed (hidden from status until a regular entry re-opens it)
goalie summary [--days N] [--user NAME|GLOB]
                                    # Entries grouped as stories per goal/task/user, last N days (default 7)
goalie status                       # Morning standup view: latest entry per user×goal×task, last 7 days
goalie update                       # Interactive end-of-day review: update tasks, log new activity
goalie export                       # Dump all data as JSONL gesture events on stdout (backup, migration, debugging)
goalie import                       # Replay a JSONL export from stdin into an initialised data repo
goalie --version                    # Print version and exit
```

### Summary output

`goalie summary` groups entries by goal, task, and user and renders each group as a chronological story. State-change labels appear only when the blocked status changes:

```
= ROUTING#impl@alice
- [Blocked] waiting for review — 5d ago
- [Unblocked] addressing changes — 4d ago
- still working through edge cases — 3d ago

= (no goal)#docs@bob
- started the ADR — 2d ago
```

## TUI

Running `goalie` opens a terminal interface with two tabs (switch with Tab / Shift-Tab):

- **Activity** — shows the latest entry per person per task across the last 30 days. Start typing to filter by note, goal, or task tag. When notifications are enabled (see [Configuration](#configuration)), this view refreshes automatically every minute; input other than `q` / Ctrl-C is locked out for the duration of the refresh.
- **Update** — shows a menu to choose what to do: review blocked tasks, log progress on a recent or new task, or edit a recent entry. ↑/↓ to move, Enter to select, Esc to return to the menu from any sub-flow, `q` to quit. Editing lets you fix the note, task tag, and blocked/done state of any entry from the last 7 days.

## Configuration

`goalie` reads optional settings from `~/.goalie/config.json`. All fields are optional; add only the ones you want to override:

```json
{
  "notifications": true
}
```

- `notifications` (bool, default `false`) — when `true`, the TUI's Activity view polls for updates every minute and fires an OS desktop notification when another user's entry newly becomes blocked, or when a note mentions you by your `@username`. Off by default; enable it by hand-editing `config.json`.
