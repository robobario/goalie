package notify

import "testing"

type fakeNotifier struct {
	title, message string
	calls          int
}

func (f *fakeNotifier) Send(title, message string) error {
	f.title, f.message = title, message
	f.calls++
	return nil
}

func TestFakeNotifierRecordsSend(t *testing.T) {
	var n Notifier = &fakeNotifier{}
	if err := n.Send("Blocked", "@alice is blocked on setup"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	f := n.(*fakeNotifier)
	if f.calls != 1 {
		t.Errorf("expected 1 call, got %d", f.calls)
	}
	if f.title != "Blocked" || f.message != "@alice is blocked on setup" {
		t.Errorf("unexpected title/message: %q / %q", f.title, f.message)
	}
}
