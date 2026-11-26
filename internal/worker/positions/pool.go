package positions

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Interval    time.Duration
	Stale       time.Duration
	Workers     int
	Queue       int
	BatchSize   int
	PushTimeout time.Duration
	Clock       func(time.Duration) (<-chan time.Time, func())
}

type Pool struct {
	log      *slog.Logger
	src      source
	push     pusher
	cfg      Config
	checks   resultCounter
	requests resultCounter
	timing   durationRecorder
	rows     rowsRecorder
	queue    *queue
	busy     atomic.Int64

	scanCancel context.CancelFunc
	workCancel context.CancelFunc
	scanDone   chan struct{}
	workDone   chan struct{}
	stopOnce   sync.Once
	stopErr    error
}

func New(
	log *slog.Logger,
	src source,
	push pusher,
	cfg Config,
	checks resultCounter,
	requests resultCounter,
	timing durationRecorder,
	rows rowsRecorder,
) *Pool {
	if cfg.Clock == nil {
		cfg.Clock = realClock
	}
	return &Pool{
		log:      log,
		src:      src,
		push:     push,
		cfg:      cfg,
		checks:   checks,
		requests: requests,
		timing:   timing,
		rows:     rows,
		queue:    newQueue(cfg.Queue),
		scanDone: make(chan struct{}),
		workDone: make(chan struct{}),
	}
}

func realClock(d time.Duration) (<-chan time.Time, func()) {
	ticker := time.NewTicker(d)
	return ticker.C, ticker.Stop
}

func (p *Pool) Start(root context.Context) {
	scanCtx, cancelScan := context.WithCancel(root)
	workCtx, cancelWork := context.WithCancel(root)
	p.scanCancel, p.workCancel = cancelScan, cancelWork
	p.log.InfoContext(root, "воркеры координат запущены", "workers", p.cfg.Workers)

	var workers sync.WaitGroup
	for range p.cfg.Workers {
		workers.Go(func() { p.worker(workCtx) })
	}
	go func() {
		workers.Wait()
		p.log.InfoContext(workCtx, "воркеры координат остановлены")
		close(p.workDone)
	}()
	go p.loop(scanCtx)
}

func (p *Pool) Stop(ctx context.Context) error {
	p.stopOnce.Do(func() {
		stop := context.AfterFunc(ctx, p.workCancel)
		defer stop()
		defer p.workCancel()
		p.scanCancel()
		<-p.scanDone
		p.queue.close()
		<-p.workDone
		p.stopErr = ctx.Err()
	})
	return p.stopErr
}

func (p *Pool) QueueSize() int64 {
	return p.queue.size()
}

func (p *Pool) WorkersBusy() int64 {
	return p.busy.Load()
}

func (p *Pool) Forget(ctx context.Context, tripID string) {
	if p.queue.forget(tripID) {
		p.requests.Add(ctx, cancelled)
	}
}
