package filesystem

import (
	"sync"
	"testing"
	"time"
)

func TestEventBus_ConcurrentPublishAndUnsubscribe(t *testing.T) {
	bus := NewEventBus()
	done := make(chan struct{})

	// Concurrently subscribe and unsubscribe
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					ch := bus.Subscribe()
					time.Sleep(time.Microsecond * 10)
					bus.Unsubscribe(ch)
				}
			}
		}()
	}

	// Concurrently publish events
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					bus.Publish(Event{Type: "updated", Path: "foo.txt"})
				}
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}
