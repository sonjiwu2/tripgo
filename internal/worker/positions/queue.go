package positions

import (
	"context"
	"slices"
	"sort"
	"sync"

	"go.opentelemetry.io/otel/trace"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

const (
	queued    = "queued"
	duplicate = "duplicate"
	dropped   = "dropped"
	cancelled = "cancelled"
)

type job struct {
	trip domaintrip.Quiet
	link trace.Link
}

type queue struct {
	mu       sync.Mutex
	jobs     []job
	capacity int
	pending  map[string]struct{}
	changed  map[string]struct{}
	ready    chan struct{}
	closed   chan struct{}
}

func newQueue(capacity int) *queue {
	return &queue{
		jobs:     make([]job, 0, capacity),
		capacity: capacity,
		pending:  make(map[string]struct{}),
		ready:    make(chan struct{}, 1),
		closed:   make(chan struct{}),
	}
}

func (q *queue) beginScan() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.changed = make(map[string]struct{})
}

func (q *queue) endScan() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.changed = nil
}

func (q *queue) put(next job) (result, droppedID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	id := next.trip.ID
	if _, ok := q.changed[id]; ok {
		return cancelled, ""
	}
	if _, ok := q.pending[id]; ok {
		return duplicate, ""
	}
	if len(q.jobs) == q.capacity {
		newest := q.jobs[0]
		if !next.trip.Since.Before(newest.trip.Since) {
			return dropped, id
		}
		q.jobs = slices.Delete(q.jobs, 0, 1)
		delete(q.pending, newest.trip.ID)
		droppedID = newest.trip.ID
	}

	idx := sort.Search(len(q.jobs), func(i int) bool {
		current := q.jobs[i].trip
		if next.trip.Since.Equal(current.Since) {
			return next.trip.ID > current.ID
		}
		return next.trip.Since.After(current.Since)
	})
	q.jobs = slices.Insert(q.jobs, idx, next)
	q.pending[id] = struct{}{}
	q.signal()
	return queued, droppedID
}

func (q *queue) forget(tripID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.changed != nil {
		q.changed[tripID] = struct{}{}
	}
	if _, ok := q.pending[tripID]; !ok {
		return false
	}
	for i, item := range q.jobs {
		if item.trip.ID == tripID {
			q.jobs = slices.Delete(q.jobs, i, i+1)
			delete(q.pending, tripID)
			return true
		}
	}
	return false
}

func (q *queue) take(ctx context.Context) (job, bool) {
	for ctx.Err() == nil {
		q.mu.Lock()
		if n := len(q.jobs); n != 0 {
			item := q.jobs[n-1]
			q.jobs = slices.Delete(q.jobs, n-1, n)
			if len(q.jobs) != 0 {
				q.signal()
			}
			q.mu.Unlock()
			return item, true
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return job{}, false
		case <-q.ready:
		case <-q.closed:
			if q.size() == 0 {
				return job{}, false
			}
		}
	}
	return job{}, false
}

func (q *queue) finish(tripID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.pending, tripID)
}

func (q *queue) signal() {
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

func (q *queue) size() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int64(len(q.jobs))
}

func (q *queue) close() {
	close(q.closed)
}
