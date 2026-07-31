package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// SessionJournal is an append-only, human-readable chronicle of a play session,
// written as events happen. Unlike the in-memory timeline (which is bounded), the
// journal keeps the full record and survives crashes because every entry is
// flushed to disk immediately.
type SessionJournal struct {
	mu sync.Mutex
	f  *os.File
}

func (s *Storage) sessionJournalPath(name string) string {
	return filepath.Join(s.basePath, SessionsDir, name+".journal.md")
}

// OpenSessionJournal opens (creating if needed) the append-only journal for a
// session and writes a run header. Close it when the session ends.
func (s *Storage) OpenSessionJournal(name string) (*SessionJournal, error) {
	dir := filepath.Join(s.basePath, SessionsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.sessionJournalPath(name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	j := &SessionJournal{f: f}
	fmt.Fprintf(f, "\n## Session started — %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	return j, nil
}

// Append writes one timeline entry to the journal.
func (j *SessionJournal) Append(e domain.LogEntry) {
	if j == nil {
		return
	}
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.f == nil {
		return
	}
	fmt.Fprintf(j.f, "- `%s` **%s** — %s\n", ts.Format("15:04:05"), e.Type, e.Message)
}

// Note writes a free-form line (e.g. oracle dialogue) that isn't part of the
// structured timeline, tagged with a kind.
func (j *SessionJournal) Note(kind, text string) {
	j.Append(domain.LogEntry{Type: domain.LogEntryType(kind), Message: text})
}

// Close flushes and closes the journal file; safe to call more than once.
func (j *SessionJournal) Close() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.f == nil {
		return nil
	}
	err := j.f.Close()
	j.f = nil
	return err
}
