package live

import "testing"

func TestSendReachesEveryConnectionOfTheUser(t *testing.T) {
	h := NewHub()
	a, stopA := h.Subscribe("u1")
	b, stopB := h.Subscribe("u1")
	other, stopOther := h.Subscribe("u2")
	defer stopA()
	defer stopB()
	defer stopOther()

	if !h.Send("u1", Event{Type: TypeMessage, MatchID: "m1"}) {
		t.Fatal("Send reported no open connection")
	}
	for _, ch := range []<-chan Event{a, b} {
		if e := <-ch; e.MatchID != "m1" {
			t.Errorf("got %+v", e)
		}
	}
	if len(other) != 0 {
		t.Error("another user got the event")
	}
	if h.Send("nobody", Event{}) {
		t.Error("Send to a user with no connection reported true")
	}
}

func TestASlowConnectionIsClosed(t *testing.T) {
	h := NewHub()
	ch, stop := h.Subscribe("u1")
	for range queueSize + 1 {
		h.Send("u1", Event{Type: TypeTyping})
	}
	n := 0
	for range ch {
		n++
	}
	if n != queueSize {
		t.Errorf("read %d events before the close, want %d", n, queueSize)
	}
	stop() // after the hub closed it: must not close twice
	if h.Send("u1", Event{}) {
		t.Error("the closed connection still counts as open")
	}
}
