Goalie helps a remote team of open source developers share their goals and surface blockers.

## How it works

Add shared goals, then create tasks using hashtags. Log updates to show whether you are blocked or making progress. Run `goalie` with no arguments to open the TUI — a two-tab interface showing team activity and a guided end-of-day update flow.

## Installation

**Prerequisites:** git.

Download the binary for your platform from the [releases page](../../releases) or build from source with `./build.sh` (requires Docker).

Copy the binary to a directory on your `PATH`:

```sh
# Linux
cp goalie-linux-amd64 ~/bin/goalie

# macOS (Apple Silicon)
cp goalie-darwin-arm64 ~/bin/goalie

# macOS (Intel)
cp goalie-darwin-amd64 ~/bin/goalie

chmod +x ~/bin/goalie
```

Then initialise goalie, pointing it at a shared repository:

```sh
goalie init https://github.com/your-org/your-repo.git
```

This clones (or connects to) the `data` branch of the repo into `~/.goalie/data`, prompts for your name, and — on a new branch — asks whether to enable client-side encryption.

**Encryption** is optional. Use it when the repository is public or semi-public and you don't want goal descriptions or journal entries readable without a key. On a private enterprise repository the repo itself provides access control, so you can skip encryption. The choice is stored in the data branch so all team members share the same setting.

If you enable encryption, `goalie init` handles key setup for the first user: it reuses an existing local key if you have one, or generates a new one. Either way it prints the hex key and commits a `key-check.enc` sentinel to the data branch:

```
Encryption key: a1b2c3...
Share with teammates: goalie key import <key>
key-check.enc committed to the data branch — teammates must import the same key.
```

When a teammate runs `goalie init`, it detects the encrypted repo and prompts for the shared key immediately:

```
Encryption key (paste hex or press Enter to skip): a1b2c3...
Encryption key verified.
```

The key is verified against `key-check.enc` before being saved. Invalid format or a mismatched key triggers an error and retries. Pressing Enter skips for now — the key can be imported later with `goalie key import <hex-key>`.

To replace a key, use `goalie key init` (generates a new key) or `goalie key import <hex>` (imports an existing one). Both commands warn you if a key file already exists, since replacing it will prevent you from decrypting data written under the old key.

By default goalie stores all data under `~/.goalie`. Set the `GOALIE_HOME` environment variable to use a different directory.

To keep goalie up to date, replace the binary with a newer build.

## Usage

```
goalie                              # Open the TUI (activity view + guided update)
goalie init <repo-url>              # Clone or create the data branch; prompts for name and encryption
goalie goal add <ID> <DESCRIPTION>  # Create a new open goal
goalie goal close <ID>              # Mark a goal as closed
goalie goal list                    # List all goals with their state
goalie log [note] [--goal ID] --task TAG [--blocked]
                                    # Append a journal entry; interactive if note is omitted. --task is required.
goalie summary [--days N] [--user NAME|GLOB]
                                    # Entries grouped as stories per goal/task/user, last N days (default 7)
goalie status                       # Morning standup view: latest entry per user×goal×task, last 7 days
goalie update                       # Interactive end-of-day review: update tasks, log new activity
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

- **Activity** — shows the latest entry per person per task across the last 30 days. Start typing to filter by note, goal, or task tag.
- **Update** — walks you through blocked tasks, recent tasks, and lets you log new activity. Press `q` to quit.
