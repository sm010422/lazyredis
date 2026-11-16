package ui

import (
	"testing"
	"time"
)

// A message can still arrive after the subscription it came from was closed
// (unsubscribe, channel switch, reconnect). Handling it used to re-arm the
// wait on a nil a.pubsub.Sub and panic inside go-redis.
func TestPubSubMessageAfterUnsubscribeDoesNotPanic(t *testing.T) {
	a, _ := newTestApp(t)
	a.pubsub.Sub = nil

	_, cmd := a.Update(msgPubSubMessage{channel: "orders", payload: "late"})
	if cmd != nil {
		cmd()
	}
}

// Switching from channel A to B closes A's subscription; A's waiter then
// reports done. That must not clear B, or B's next message hits the panic above.
func TestStalePubSubDoneKeepsCurrentSubscription(t *testing.T) {
	a, _ := newTestApp(t)
	sub := a.redis.Subscribe("b")
	t.Cleanup(func() { _ = sub.Close() })
	a.pubsub.Sub = sub
	a.pubsub.SubChannel = "b"

	a.Update(msgPubSubDone{})

	if a.pubsub.Sub != sub || a.pubsub.SubChannel != "b" {
		t.Fatalf("current subscription was cleared by a stale done message: sub=%v channel=%q", a.pubsub.Sub, a.pubsub.SubChannel)
	}
}

// The normal path still works: a message from the current subscription is
// shown and the wait is re-armed.
func TestPubSubMessageFromCurrentSubscriptionIsShown(t *testing.T) {
	a, m := newTestApp(t)
	sub := a.redis.Subscribe("orders")
	t.Cleanup(func() { _ = sub.Close() })
	a.pubsub.Sub = sub
	a.pubsub.SubChannel = "orders"

	got := make(chan any, 1)
	wait := a.waitPubSubMsg()
	go func() { got <- wait() }()

	deadline := time.After(2 * time.Second)
	for {
		m.Publish("orders", "paid")
		select {
		case msg := <-got:
			_, next := a.Update(msg)
			if len(a.pubsub.Messages) != 1 || a.pubsub.Messages[0].Payload != "paid" {
				t.Fatalf("messages = %+v, want one \"paid\"", a.pubsub.Messages)
			}
			if next == nil {
				t.Fatal("wait was not re-armed after a message")
			}
			return
		case <-deadline:
			t.Fatal("no message received")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
