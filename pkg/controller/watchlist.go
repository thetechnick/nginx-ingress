package controller

import "sync"

// The Watchlist coordinates watch requests of multiple watchers
// items are discarded when there are no more watchers
type Watchlist interface {
	Add(key string, watcher string)
	Remove(watcher string)
	Watchers(key string) []string
}

// NewWatchlist creates a new watchlist
func NewWatchlist() Watchlist {
	return &watchlist{
		watch: map[string]map[string]bool{},
	}
}

type watchlist struct {
	watch map[string]map[string]bool
	lock  sync.RWMutex
}

func (w *watchlist) Add(key string, watcher string) {
	w.lock.Lock()
	defer w.lock.Unlock()

	if list, exists := w.watch[key]; exists {
		list[watcher] = true
		return
	}
	w.watch[key] = map[string]bool{
		watcher: true,
	}
}

func (w *watchlist) Remove(watcher string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	for key, watchers := range w.watch {
		delete(watchers, watcher)
		if len(watchers) == 0 {
			delete(w.watch, key)
		}
	}
}

func (w *watchlist) Watchers(key string) []string {
	w.lock.RLock()
	defer w.lock.RUnlock()
	watchersList := []string{}
	if watchers, onList := w.watch[key]; onList {
		for watcher := range watchers {
			watchersList = append(watchersList, watcher)
		}
	}
	return watchersList
}
