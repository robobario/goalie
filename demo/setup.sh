#!/usr/bin/env bash
set -euo pipefail

REMOTE=/demo-remote
HOME_ALICE=/demo-home-alice
HOME_BOB=/demo-home-bob
HOME_CAROL=/demo-home-carol
HOME_DAN=/demo-home-dan

git init --bare "$REMOTE"

export GOALIE_HOME="$HOME_ALICE"
# Alice creates the data branch (new branch → encryption prompt + username prompt)
printf 'n\nalice\n' | goalie init "file://$REMOTE"

# Goals are shared team-wide; add them as alice
goalie goal add PLATFORM "Migrate services to Kubernetes"
goalie goal add ONBOARDING "Improve developer onboarding"
goalie goal add RELIABILITY "Achieve 99.9% uptime for core APIs"
goalie goal add INCIDENTS "Reduce mean time to resolution"

goalie motd set "Sprint planning Thursday 2pm — ping @alice if you need to reschedule"

# Alice's entries
goalie log "Finished Helm charts for the auth service" --goal PLATFORM --task "#k8s-migration"
goalie log "Updated README with local dev environment steps" --goal ONBOARDING --task "#docs"

export GOALIE_HOME="$HOME_BOB"
# Bob clones (data branch already exists → username prompt only)
printf 'bob\n' | goalie init "file://$REMOTE"
goalie log "Added alert rules for API error rate spikes" --goal RELIABILITY --task "#monitoring"
goalie log "Reviewed Alice's Helm charts, left comments on resource limits" --goal PLATFORM --task "#k8s-migration"
goalie log "Finished postmortem write-up for last week's DB failover" --goal INCIDENTS --task "#postmortem"

export GOALIE_HOME="$HOME_CAROL"
printf 'carol\n' | goalie init "file://$REMOTE"
goalie log "Recorded walkthrough video for new hire setup" --goal ONBOARDING --task "#video"
goalie log "Updated SLO dashboard with new latency targets" --goal RELIABILITY --task "#slo"

export GOALIE_HOME="$HOME_DAN"
printf 'dan\n' | goalie init "file://$REMOTE"
goalie log "Migrated staging environment, all services healthy" --goal PLATFORM --task "#k8s-migration"
goalie log "Fixed false positive on memory alert, need bob to confirm in prod" --goal RELIABILITY --task "#monitoring" --blocked

# Pull so that alice's local checkout has everyone's journal entries before the TUI starts.
# CollectLatest also pulls on startup, but having it ready avoids a slow first-render.
git -C "$HOME_ALICE/data" pull
