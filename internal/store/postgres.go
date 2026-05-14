package store

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			epay_trade_no TEXT NOT NULL,
			vmq_order_id TEXT NOT NULL DEFAULT '',
			alipay_trade_no TEXT NOT NULL DEFAULT '',
			amount TEXT NOT NULL,
			original_amount TEXT NOT NULL,
			subject TEXT NOT NULL DEFAULT '',
			notify_url TEXT NOT NULL DEFAULT '',
			return_url TEXT NOT NULL DEFAULT '',
			token TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			paid_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_orders_epay_trade_no ON orders(epay_trade_no);
		CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);

		CREATE TABLE IF NOT EXISTS notifications (
			id BIGSERIAL PRIMARY KEY,
			alipay_trade_no TEXT NOT NULL,
			order_id TEXT NOT NULL,
			raw_body TEXT NOT NULL,
			received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_notifications_alipay_trade_no ON notifications(alipay_trade_no);
	`)
	return err
}

func (s *PostgresStore) CreateOrder(ctx context.Context, order *Order) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO orders (id, epay_trade_no, vmq_order_id, amount, original_amount, subject, notify_url, return_url, token, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		order.ID, order.EpayTradeNo, order.VMQOrderID, order.Amount, order.OriginalAmount,
		order.Subject, order.NotifyURL, order.ReturnURL, order.Token, order.Status, order.CreatedAt,
	)
	return err
}

func (s *PostgresStore) GetOrder(ctx context.Context, id string) (*Order, error) {
	return s.scanOrder(s.db.QueryRowContext(ctx,
		`SELECT id, epay_trade_no, vmq_order_id, alipay_trade_no, amount, original_amount, subject, notify_url, return_url, token, status, created_at, paid_at
		 FROM orders WHERE id = $1`, id))
}

func (s *PostgresStore) GetOrderByEpayTradeNo(ctx context.Context, tradeNo string) (*Order, error) {
	return s.scanOrder(s.db.QueryRowContext(ctx,
		`SELECT id, epay_trade_no, vmq_order_id, alipay_trade_no, amount, original_amount, subject, notify_url, return_url, token, status, created_at, paid_at
		 FROM orders WHERE epay_trade_no = $1`, tradeNo))
}

func (s *PostgresStore) UpdateAlipayTradeNo(ctx context.Context, orderID, tradeNo string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET alipay_trade_no = $1 WHERE id = $2`, tradeNo, orderID)
	return err
}

func (s *PostgresStore) MarkPaid(ctx context.Context, orderID string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET status = $1, paid_at = $2 WHERE id = $3 AND status = $4`,
		StatusPaid, now, orderID, StatusPending)
	return err
}

func (s *PostgresStore) MarkExpired(ctx context.Context, orderID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET status = $1 WHERE id = $2 AND status = $3`,
		StatusExpired, orderID, StatusPending)
	return err
}

func (s *PostgresStore) ListPendingOrders(ctx context.Context, olderThan int) ([]*Order, error) {
	cutoff := time.Now().Add(-time.Duration(olderThan) * time.Second)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, epay_trade_no, vmq_order_id, alipay_trade_no, amount, original_amount, subject, notify_url, return_url, token, status, created_at, paid_at
		 FROM orders WHERE status = $1 AND created_at < $2`, StatusPending, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		o, err := s.scanOrderFromRows(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (s *PostgresStore) SaveNotification(ctx context.Context, n *Notification) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notifications (alipay_trade_no, order_id, raw_body, received_at) VALUES ($1, $2, $3, $4)`,
		n.AlipayTradeNo, n.OrderID, n.RawBody, n.ReceivedAt)
	return err
}

func (s *PostgresStore) HasNotification(ctx context.Context, alipayTradeNo string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE alipay_trade_no = $1`, alipayTradeNo).Scan(&count)
	return count > 0, err
}

func (s *PostgresStore) scanOrder(row *sql.Row) (*Order, error) {
	o := &Order{}
	err := row.Scan(&o.ID, &o.EpayTradeNo, &o.VMQOrderID, &o.AlipayTradeNo, &o.Amount,
		&o.OriginalAmount, &o.Subject, &o.NotifyURL, &o.ReturnURL, &o.Token, &o.Status,
		&o.CreatedAt, &o.PaidAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (s *PostgresStore) scanOrderFromRows(rows *sql.Rows) (*Order, error) {
	o := &Order{}
	err := rows.Scan(&o.ID, &o.EpayTradeNo, &o.VMQOrderID, &o.AlipayTradeNo, &o.Amount,
		&o.OriginalAmount, &o.Subject, &o.NotifyURL, &o.ReturnURL, &o.Token, &o.Status,
		&o.CreatedAt, &o.PaidAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}
