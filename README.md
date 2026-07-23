Goalie helps a remote team of open source developers share their goals and surface blockers.

## How it works

Add shared goals, then create threads of work using hashtags. Log updates to show whether you are blocked or making progress. Run `goalie` with no arguments to open the TUI — a two-tab interface showing team activity and a guided end-of-day update flow.

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

This clones (or connects to) the `data` branch of the repo into `~/.goalie/data` and prompts you for your name.

To keep goalie up to date, replace the binary with a newer build.

## Usage

```
goalie                              # Open the TUI (activity view + guided update)
goalie init <repo-url>              # Clone or create the data branch in ~/.goalie/data
goalie goal add <ID> <DESCRIPTION>  # Create a new open goal
goalie goal close <ID>              # Mark a goal as closed
goalie goal list                    # List all goals with their state
goalie log [note] [--goal ID] [--thread TAG] [--blocked]
                                    # Append a journal entry; interactive if note is omitted
goalie summary [--days N] [--user NAME|GLOB]
                                    # Your entries for the last N days (default 7); --user '*' for everyone
goalie status                       # Morning standup view: latest entry per user×goal×thread, last 7 days
goalie update                       # Interactive end-of-day review: update threads, log new activity
goalie --version                    # Print version and exit
```

## TUI

Running `goalie` opens a terminal interface with two tabs (switch with Tab / Shift-Tab):

- **Activity** — shows the latest entry per person per thread across the last 30 days. Start typing to filter by note, goal, or thread tag.
- **Update** — walks you through blocked threads, recent threads, and lets you log new activity. Press `q` to quit.
