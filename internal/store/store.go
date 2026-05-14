package store

import "context"

type Store interface {
	Init(ctx context.Context) error
	CreateOrder(ctx context.Context, order *Order) error
	GetOrder(ctx context.Context, id string) (*Order, error)
	GetOrderByEpayTradeNo(ctx context.Context, tradeNo string) (*Order, error)
	UpdateAlipayTradeNo(ctx context.Context, orderID, tradeNo string) error
	MarkPaid(ctx context.Context, orderID string) error
	MarkExpired(ctx context.Context, orderID string) error
	ListPendingOrders(ctx context.Context, olderThan int) ([]*Order, error) // olderThan in seconds
	SaveNotification(ctx context.Context, n *Notification) error
	HasNotification(ctx context.Context, alipayTradeNo string) (bool, error)
}
