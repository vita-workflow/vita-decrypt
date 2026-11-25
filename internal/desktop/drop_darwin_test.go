package desktop

import (
	"strings"
	"testing"
)

func TestDropQueue_DeliversEventsInOrder(t *testing.T) {
	events := make(chan string, dropQueueLength)
	queue := newDropQueue()
	queue.handler = DropHandler{
		Enter: func() { events <- "enter" },
		Leave: func() { events <- "leave" },
		Drop:  func(paths []string) { events <- "drop " + strings.Join(paths, ",") },
	}
	queue.enter()
	queue.drop("\n")
	queue.drop("/Users/a/x.age\n\n/Users/a/y z.age\n")
	queue.leave()
	for _, want := range []string{"enter", "drop /Users/a/x.age,/Users/a/y z.age", "leave"} {
		if event := <-events; event != want {
			t.Fatalf("got the event %q, want %q", event, want)
		}
	}
}

func TestDropQueue_WithoutAHandlerQueuesNothing(t *testing.T) {
	queue := &dropQueue{events: make(chan func(), dropQueueLength)} // nothing reads the events
	queue.enter()
	queue.drop("/Users/a/x.age")
	queue.leave()
	if queued := len(queue.events); queued != 0 {
		t.Fatalf("%d events were queued", queued)
	}
}
