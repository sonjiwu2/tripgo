package positions

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (p *Pool) worker(ctx context.Context) {
	for {
		item, ok := p.queue.take(ctx)
		if !ok {
			return
		}
		p.handle(ctx, item)
		p.queue.finish(item.trip.ID)
	}
}

func (p *Pool) handle(ctx context.Context, item job) {
	p.busy.Add(1)
	defer p.busy.Add(-1)

	ctx, cancel := context.WithTimeout(ctx, p.cfg.PushTimeout)
	defer cancel()
	ctx, span := otel.Tracer("github.com/sonjiwu2/tripgo").Start(ctx, "positions.task", trace.WithNewRoot(), trace.WithLinks(item.link))
	defer span.End()

	started := time.Now()
	err := p.push.RequestPosition(ctx, item.trip.DriverID, item.trip.ID)
	result := "success"
	if err != nil {
		result = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, "запрос координаты не удался")
		p.log.WarnContext(ctx, "push не принял запрос координаты", "trip_id", item.trip.ID, "err", err)
	}
	p.requests.Add(ctx, result)
	p.timing.Record(ctx, time.Since(started).Seconds(), result)
}
