package positions

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

type (
	source interface {
		Quiet(ctx context.Context, before time.Time) ([]domaintrip.Quiet, error)
	}

	pusher interface {
		RequestPosition(ctx context.Context, driverID, tripID string) error
	}

	resultCounter interface {
		Add(ctx context.Context, result string)
	}

	durationRecorder interface {
		Record(ctx context.Context, seconds float64, result string)
	}
)

type Config struct {
	Interval    time.Duration
	Stale       time.Duration
	Workers     int
	Queue       int
	PushTimeout time.Duration
	Clock       func(time.Duration) (<-chan time.Time, func())
}

type job struct {
	tripID   string
	driverID string
	link     trace.Link
}

type Pool struct {
	log         *slog.Logger
	src         source
	push        pusher
	stale       time.Duration
	interval    time.Duration
	pushTimeout time.Duration
	workers     int
	open        func(time.Duration) (<-chan time.Time, func())
	checks      resultCounter
	requests    resultCounter
	timing      durationRecorder

	jobs    chan job
	mu      sync.Mutex
	waiting map[string]struct{}
	busy    atomic.Int64

	scanCtx    context.Context
	scanCancel context.CancelFunc
	workCtx    context.Context
	workCancel context.CancelFunc
	scanDone   chan struct{}
	wg         sync.WaitGroup
	stopOnce   sync.Once
}

func New(
	log *slog.Logger,
	src source,
	push pusher,
	cfg Config,
	checks resultCounter,
	requests resultCounter,
	timing durationRecorder,
) *Pool {
	open := cfg.Clock
	if open == nil {
		open = realClock
	}
	return &Pool{
		log:         log,
		src:         src,
		push:        push,
		stale:       cfg.Stale,
		interval:    cfg.Interval,
		pushTimeout: cfg.PushTimeout,
		workers:     cfg.Workers,
		open:        open,
		checks:      checks,
		requests:    requests,
		timing:      timing,
		jobs:        make(chan job, cfg.Queue),
		waiting:     map[string]struct{}{},
	}
}

func realClock(d time.Duration) (<-chan time.Time, func()) {
	ticker := time.NewTicker(d)
	return ticker.C, ticker.Stop
}

func (p *Pool) Start(root context.Context) {
	p.scanCtx, p.scanCancel = context.WithCancel(root)
	p.workCtx, p.workCancel = context.WithCancel(root)
	p.scanDone = make(chan struct{})
	p.log.Info("воркеры координат запущены", "workers", p.workers)
	for range p.workers {
		p.wg.Add(1)
		go p.worker()
	}
	go p.loop()
}

func (p *Pool) Stop(ctx context.Context) error {
	var stopErr error
	p.stopOnce.Do(func() {
		p.scanCancel()
		<-p.scanDone
		close(p.jobs)
		finished := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(finished)
		}()
		select {
		case <-finished:
			p.log.Info("опрос координат остановлен")
		case <-ctx.Done():
			p.workCancel()
			<-finished
			p.log.Warn("воркеры координат не успели доработать очередь")
			stopErr = ctx.Err()
		}
	})
	return stopErr
}

func (p *Pool) QueueSize() int64 {
	return int64(len(p.jobs))
}

func (p *Pool) WorkersBusy() int64 {
	return p.busy.Load()
}

func (p *Pool) Forget(tripID string) {
	p.mu.Lock()
	delete(p.waiting, tripID)
	p.mu.Unlock()
}

func (p *Pool) Put(ctx context.Context, item domaintrip.Quiet, link trace.Link) string {
	p.mu.Lock()
	if _, ok := p.waiting[item.ID]; ok {
		p.mu.Unlock()
		p.checks.Add(ctx, "duplicate")
		return "duplicate"
	}
	p.waiting[item.ID] = struct{}{}
	p.mu.Unlock()

	next := job{tripID: item.ID, driverID: item.DriverID, link: link}
	select {
	case p.jobs <- next:
		p.checks.Add(ctx, "queued")
		return "queued"
	default:
		p.mu.Lock()
		delete(p.waiting, item.ID)
		p.mu.Unlock()
		p.checks.Add(ctx, "dropped")
		p.requests.Add(ctx, "dropped")
		p.log.WarnContext(ctx, "очередь координат заполнена, поездку пропускаю", "trip_id", item.ID)
		return "dropped"
	}
}
