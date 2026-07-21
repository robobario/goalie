This repository is intended to help a remote team of Open Source Software Developers achieve their shared Goals!

## Goalie

Goalie is a way of sharing what is blocking you. We add shared Goals, then create Threads of work using hashtags. We update the threads to show if we are blocked or unblocked.

`goalie update` is an interactive way to record your status at the end of the day
`goalie status` is how you find out whether your team is blocked
`goalie summary` is how you get a summary of your week, handy for that blimmin report


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

Then initialise goalie, pointing it at the repo over SSH:

```sh
goalie init git@github.com:robobario/goalie.git
```

This clones (or connects to) the `data` branch of the repo into `~/.goalie/data` and prompts you for your name.

To keep goalie up to date, replace the binary with a newer build.

## Usage

```
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
```
