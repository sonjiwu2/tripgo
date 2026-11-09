package positions

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.workCtx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			if !p.still(job.tripID) {
				continue
			}
			p.handle(job)
			p.finish(job.tripID)
		}
	}
}

func (p *Pool) still(tripID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.waiting[tripID]
	return ok
}

func (p *Pool) finish(tripID string) {
	p.mu.Lock()
	delete(p.waiting, tripID)
	p.mu.Unlock()
}

func (p *Pool) handle(job job) {
	p.busy.Add(1)
	defer p.busy.Add(-1)

	ctx, cancel := context.WithTimeout(p.workCtx, p.pushTimeout)
	defer cancel()
	ctx, span := otel.Tracer("github.com/sonjiwu2/tripgo").Start(ctx, "positions.task", trace.WithLinks(job.link))
	defer span.End()

	started := time.Now()
	err := p.push.RequestPosition(ctx, job.driverID, job.tripID)
	result := "success"
	if err != nil {
		result = "error"
		p.log.WarnContext(ctx, "push не принял запрос координаты", "trip_id", job.tripID, "err", err)
	}
	p.requests.Add(ctx, result)
	p.timing.Record(ctx, time.Since(started).Seconds(), result)
}
