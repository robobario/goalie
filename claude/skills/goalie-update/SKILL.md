Log progress to goalie for the current session.

Arguments (`$ARGUMENTS`) may include any combination of:
- A task tag: starts with `#`, lowercase (e.g. `#impl`, `#build-fix`) — use this as `--task`
- A goal ID: all uppercase letters/digits/underscores (e.g. `MYGOAL`, `Q3_INFRA`) — use this as `--goal`
- The word `compact` — trigger `/compact` after logging

1. Parse `$ARGUMENTS` for a task tag, goal ID, and whether `compact` was requested.

2. If no task tag was supplied, look at the conversation context to infer the most relevant one. If still ambiguous, ask the user to confirm before proceeding. Same for goal ID — infer from context if not supplied, skip `--goal` if none is apparent.

3. Summarise in one sentence the work done since the last goalie log entry this session (or the whole session if this is the first). Be specific about what changed.

4. Run:
   ```
   goalie log --task <tag> [--goal <goal-id>] "<summary>"
   ```

5. If `compact` was requested, run `/compact`.
