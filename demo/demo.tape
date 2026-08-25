Output "/output/goalie-demo.gif"

Set Width 1200
Set Height 675
Set FontSize 14
Set Padding 10
Set Theme "Dracula"
Set Shell bash
Set TypingSpeed 80ms

# Launch goalie — lands on Activity tab showing the whole team's recent work
Type "goalie"
Sleep 200ms
Enter

# Let the viewer read the team's activity feed
Sleep 2s

# Switch to Update tab (Ctrl+Right toggles between the two tabs)
Ctrl+Shift+Right
Sleep 2s

# Menu: cursor starts at "Update a task" (index 0)
# Move down to "Log progress on a new task" (index 1)
Down
Sleep 200ms
Enter
Sleep 1000ms

# Goal picker: type to filter, select RELIABILITY
Type "RELI"
Sleep 500ms
Enter
Sleep 500ms

# Task field: type the tag body (# is auto-prepended by the TUI)
Type "monitoring"
Sleep 500ms
Enter
Sleep 500ms

# Note field: type a progress note with @mention autocompletion
Type "Checked @da"
Sleep 200ms
Tab
Sleep 200ms
Type "'s alert, confirmed false positive in prod"
Sleep 300ms
Enter
Sleep 300ms

# Blocked? n = not blocked → submits entry and triggers git commit+push
Type "n"

# Wait for the async git operations to complete before continuing
Sleep 1s

# Back at menu (cursor still at index 1 from before)
# Move up to "Update a task" (index 0)
Up
Sleep 300ms
Enter
Sleep 1.2s

# Task picker shows alice's tasks sorted by recency:
#   index 0: #monitoring (RELIABILITY) — just created
#   index 1: #k8s-migration (PLATFORM)
#   index 2: #docs (ONBOARDING)
# Navigate down twice to #k8s-migration
Down
Sleep 100ms
Enter
Sleep 500ms

# Note field: explain why this task is now blocked
Type "Waiting for security sign-off before prod deploy"
Sleep 300ms
Enter
Sleep 300ms

# State selector starts at [Unblocked] because the task is currently unblocked.
# Press Left to move to [Blocked], then Enter to submit.
Left
Sleep 300ms
Enter

# Switch back to Activity tab — the updated entries are visible after the pull

Ctrl+Shift+Left
Sleep 2s

# Switch back to Update tab and unblock dan

Ctrl+Shift+Right
Sleep 1s

Down
Sleep 50ms
Down
Sleep 50ms
Down
Sleep 50ms
Enter
Sleep 1s
Enter
Type "I've reviewed and the alert is a false positive"
Sleep 200ms
Enter
Ctrl+Shift+Left
Sleep 4s
