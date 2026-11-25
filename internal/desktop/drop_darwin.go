package desktop

// dropQueueLength is how many drag events can wait for the handler before AppKit's thread blocks.
const dropQueueLength = 64

// dropQueue carries drag events from AppKit to one goroutine that calls the handler in order. The handler must not
// run on the AppKit thread: Gio's Window.Invalidate, called from there, re-enters Gio's event loop while holding a
// lock it then takes again, which hangs the app.
type dropQueue struct {
	handler DropHandler
	events  chan func()
}

// newDropQueue makes a queue and starts the goroutine that calls its handler.
func newDropQueue() *dropQueue {
	queue := &dropQueue{events: make(chan func(), dropQueueLength)}
	go queue.deliver()
	return queue
}

func (queue *dropQueue) deliver() {
	for event := range queue.events {
		event()
	}
}

// enter reports that files were dragged over the window.
func (queue *dropQueue) enter() {
	if queue.handler.Enter != nil {
		queue.events <- queue.handler.Enter
	}
}

// leave reports that the drag left the window or ended.
func (queue *dropQueue) leave() {
	if queue.handler.Leave != nil {
		queue.events <- queue.handler.Leave
	}
}

// drop reports the dropped files, given as newline-separated paths.
func (queue *dropQueue) drop(text string) {
	paths, handleDrop := nonEmptyLines(text), queue.handler.Drop
	if len(paths) == 0 || handleDrop == nil {
		return
	}
	queue.events <- func() { handleDrop(paths) }
}
