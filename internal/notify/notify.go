// Package notify sends OS desktop notifications for the TUI's periodic
// Activity refresh (see internal/tui). The Notifier interface lets callers
// substitute a fake in tests instead of triggering real OS popups.
package notify

import "github.com/gen2brain/beeep"

type Notifier interface {
	Send(title, message string) error
}

type BeeepNotifier struct{}

func (BeeepNotifier) Send(title, message string) error {
	return beeep.Notify(title, message, "")
}
