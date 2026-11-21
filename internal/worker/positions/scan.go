package positions

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func (p *Pool) loop() {
	defer close(p.scanDone)
	ticks, stop := p.open(p.interval)
	defer stop()
	p.log.Info("сканер координат запущен", "interval", p.interval.String())
	for {
		select {
		case <-p.scanCtx.Done():
			p.log.Info("сканер координат останавливается")
			return
		case <-ticks:
			p.pass(p.scanCtx)
		}
	}
}

func (p *Pool) pass(ctx context.Context) {
	ctx, span := otel.Tracer("github.com/sonjiwu2/tripgo").Start(ctx, "positions.scan")
	defer span.End()

	rows, err := p.src.Quiet(ctx, time.Now().Add(-p.stale))
	if err != nil {
		p.log.ErrorContext(ctx, "не получилось выбрать поездки без координат", "err", err)
		return
	}
	link := trace.Link{SpanContext: span.SpanContext()}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		p.Put(ctx, row, link)
	}
}
