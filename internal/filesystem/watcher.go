package filesystem

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

func startWatcher(root string, events *EventBus) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	var addRecursive func(string) error
	addRecursive = func(dir string) error {
		return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if filepath.Base(path) == ".git" {
					return filepath.SkipDir
				}
				err = watcher.Add(path)
				if err != nil {
					log.Printf("Failed to watch %s: %v", path, err)
				}
			}
			return nil
		})
	}

	if err := addRecursive(root); err != nil {
		watcher.Close()
		return nil, err
	}

	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				eventType := "updated"
				if event.Has(fsnotify.Create) {
					eventType = "created"
					info, err := os.Stat(event.Name)
					if err == nil && info.IsDir() {
						_ = addRecursive(event.Name)
					}
				} else if event.Has(fsnotify.Remove) {
					eventType = "deleted"
				} else if event.Has(fsnotify.Rename) {
					eventType = "renamed"
				}

				rel, err := filepath.Rel(root, event.Name)
				if err == nil {
					logicalPath := filepath.ToSlash(rel)
					if logicalPath == "." {
						logicalPath = ""
					}
					if !strings.HasPrefix(logicalPath, ".git") {
						events.Publish(Event{Type: eventType, Path: logicalPath})
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	stopFunc := func() {
		close(done)
		watcher.Close()
	}

	return stopFunc, nil
}
