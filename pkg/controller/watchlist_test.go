package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWatchlist(t *testing.T) {
	assert := assert.New(t)
	w := NewWatchlist()

	assert.Len(w.Watchers("key1"), 0, "key1 is not on the list by default")

	w.Add("key1", "watcher1")
	w.Add("key1", "watcher1")
	w.Add("key1", "watcher2")
	assert.Len(w.Watchers("key1"), 2, "key1 should have 2 watchers")

	w.Remove("watcher1")
	assert.Len(w.Watchers("key1"), 1, "key1 should still have 1 watcher")

	w.Remove("watcher2")
	assert.Len(w.Watchers("key1"), 0, "key1 should have no watchers anymore")
}
