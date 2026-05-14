package poller

import (
	"context"
	"log/slog"
	"time"

	"alipay-vmq/internal/store"
)

type QueryPusher interface {
	TryQueryAndPush(ctx context.Context, order *store.Order)
}

type Poller struct {
	store    store.Store
	pusher   QueryPusher
	interval time.Duration
	minAge   int // seconds before polling a pending order
}

func New(st store.Store, pusher QueryPusher, intervalSec int) *Poller {
	return &Poller{
		store:    st,
		pusher:   pusher,
		interval: time.Duration(intervalSec) * time.Second,
		minAge:   120,
	}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	orders, err := p.store.ListPendingOrders(ctx, p.minAge)
	if err != nil {
		slog.Error("poller list pending orders", "error", err)
		return
	}

	for _, order := range orders {
		p.pusher.TryQueryAndPush(ctx, order)
	}
}
