// Copied from github.com/d2lang/dagro, file list.go, at commit
// 30476a97eb11 (D39, D44, docs/UPSTREAM.md). Its MIT license is in
// internal/dagre/LICENSE, and its notice in internal/dagre/NOTICE.

package dagro

import (
	// zuzu: change 3. The copy reads encoding/json/v2, which TinyGo
	// builds, in place of encoding/json (D29).
	"encoding/json/v2"
	"strings"
)

type fasEntry struct {
	V           string
	In, Out     float64
	data        any
	prev, next  *fasEntry
	currentList *fasList
}

type fasList struct {
	head, tail *fasEntry
}

func (l *fasList) dequeue() *fasEntry {
	entry := l.tail
	if entry != nil {
		l.unlink(entry)
	}
	return entry
}

func (l *fasList) enqueue(entry *fasEntry) {
	if entry.currentList != nil {
		entry.currentList.unlink(entry)
	}
	entry.currentList = l
	entry.prev = nil
	entry.next = l.head
	if l.head != nil {
		l.head.prev = entry
	} else {
		l.tail = entry
	}
	l.head = entry
}

func (l *fasList) unlink(entry *fasEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		l.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		l.tail = entry.prev
	}
	entry.prev, entry.next, entry.currentList = nil, nil, nil
}

func (l *fasList) String() string {
	items := make([]string, 0)
	for entry := l.tail; entry != nil; entry = entry.prev {
		value := entry.data
		if value == nil {
			value = map[string]any{"v": entry.V, "in": entry.In, "out": entry.Out}
		}
		// zuzu: change 3. encoding/json sorts the keys of a map, and
		// encoding/json/v2 does so only with the option Deterministic.
		encoded, _ := json.Marshal(value, json.Deterministic(true))
		items = append(items, string(encoded))
	}
	return "[" + strings.Join(items, ", ") + "]"
}
