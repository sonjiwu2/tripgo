package positions

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	domaintrip "github.com/sonjiwu2/tripgo/internal/domain/trip"
)

func (p *Pool) loop(ctx context.Context) {
	defer close(p.scanDone)
	ticks, stop := p.cfg.Clock(p.cfg.Interval)
	defer stop()
	p.log.InfoContext(ctx, "сканер координат запущен", "interval", p.cfg.Interval.String())
	defer p.log.InfoContext(ctx, "сканер координат остановлен")
	for {
		select {
		case <-ctx.Done():
			return
		case at := <-ticks:
			p.pass(ctx, at)
		}
	}
}

func (p *Pool) pass(ctx context.Context, at time.Time) {
	ctx, span := otel.Tracer("github.com/sonjiwu2/tripgo").Start(ctx, "positions.scan")
	defer span.End()

	p.queue.beginScan()
	defer p.queue.endScan()
	var read int64
	defer func() { p.rows.Record(ctx, read) }()

	link := trace.Link{SpanContext: span.SpanContext()}
	var after domaintrip.Quiet
	for ctx.Err() == nil {
		rows, err := p.src.Quiet(ctx, at, p.cfg.Stale, after, p.cfg.BatchSize)
		if err != nil {
			if ctx.Err() == nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "выборка поездок не удалась")
				p.log.ErrorContext(ctx, "не получилось выбрать поездки без координат", "err", err)
			}
			return
		}
		read += int64(len(rows))
		if !p.enqueue(ctx, rows, link) || len(rows) < p.cfg.BatchSize {
			return
		}
		after = rows[len(rows)-1]
	}
}

func (p *Pool) enqueue(ctx context.Context, rows []domaintrip.Quiet, link trace.Link) bool {
	for _, row := range rows {
		if ctx.Err() != nil {
			return false
		}
		result, droppedID := p.queue.put(job{trip: row, link: link})
		switch result {
		case queued, duplicate:
			p.checks.Add(ctx, result)
		}
		if droppedID != "" {
			p.checks.Add(ctx, dropped)
			p.log.WarnContext(ctx, "очередь координат заполнена, поездку пропускаю", "trip_id", droppedID)
			return false
		}
	}
	return true
}
