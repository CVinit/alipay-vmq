package store

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Init(ctx context.Context) error {
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
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			paid_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_orders_epay_trade_no ON orders(epay_trade_no);
		CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);

		CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alipay_trade_no TEXT NOT NULL,
			order_id TEXT NOT NULL,
			raw_body TEXT NOT NULL,
			received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_notifications_alipay_trade_no ON notifications(alipay_trade_no);
	`)
	return err
}

func (s *SQLiteStore) CreateOrder(ctx context.Context, order *Order) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO orders (id, epay_trade_no, vmq_order_id, amount, original_amount, subject, notify_url, return_url, token, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.ID, order.EpayTradeNo, order.VMQOrderID, order.Amount, order.OriginalAmount,
		order.Subject, order.NotifyURL, order.ReturnURL, order.Token, order.Status, order.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) GetOrder(ctx context.Context, id string) (*Order, error) {
	return s.scanOrder(s.db.QueryRowContext(ctx,
		`SELECT id, epay_trade_no, vmq_order_id, alipay_trade_no, amount, original_amount, subject, notify_url, return_url, token, status, created_at, paid_at
		 FROM orders WHERE id = ?`, id))
}

func (s *SQLiteStore) GetOrderByEpayTradeNo(ctx context.Context, tradeNo string) (*Order, error) {
	return s.scanOrder(s.db.QueryRowContext(ctx,
		`SELECT id, epay_trade_no, vmq_order_id, alipay_trade_no, amount, original_amount, subject, notify_url, return_url, token, status, created_at, paid_at
		 FROM orders WHERE epay_trade_no = ?`, tradeNo))
}

func (s *SQLiteStore) UpdateAlipayTradeNo(ctx context.Context, orderID, tradeNo string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET alipay_trade_no = ? WHERE id = ?`, tradeNo, orderID)
	return err
}

func (s *SQLiteStore) MarkPaid(ctx context.Context, orderID string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET status = ?, paid_at = ? WHERE id = ? AND status = ?`,
		StatusPaid, now, orderID, StatusPending)
	return err
}

func (s *SQLiteStore) MarkExpired(ctx context.Context, orderID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET status = ? WHERE id = ? AND status = ?`,
		StatusExpired, orderID, StatusPending)
	return err
}

func (s *SQLiteStore) ListPendingOrders(ctx context.Context, olderThan int) ([]*Order, error) {
	cutoff := time.Now().Add(-time.Duration(olderThan) * time.Second)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, epay_trade_no, vmq_order_id, alipay_trade_no, amount, original_amount, subject, notify_url, return_url, token, status, created_at, paid_at
		 FROM orders WHERE status = ? AND created_at < ?`, StatusPending, cutoff)
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

func (s *SQLiteStore) SaveNotification(ctx context.Context, n *Notification) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notifications (alipay_trade_no, order_id, raw_body, received_at) VALUES (?, ?, ?, ?)`,
		n.AlipayTradeNo, n.OrderID, n.RawBody, n.ReceivedAt)
	return err
}

func (s *SQLiteStore) HasNotification(ctx context.Context, alipayTradeNo string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE alipay_trade_no = ?`, alipayTradeNo).Scan(&count)
	return count > 0, err
}

func (s *SQLiteStore) scanOrder(row *sql.Row) (*Order, error) {
	o := &Order{}
	err := row.Scan(&o.ID, &o.EpayTradeNo, &o.VMQOrderID, &o.AlipayTradeNo, &o.Amount,
		&o.OriginalAmount, &o.Subject, &o.NotifyURL, &o.ReturnURL, &o.Token, &o.Status,
		&o.CreatedAt, &o.PaidAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (s *SQLiteStore) scanOrderFromRows(rows *sql.Rows) (*Order, error) {
	o := &Order{}
	err := rows.Scan(&o.ID, &o.EpayTradeNo, &o.VMQOrderID, &o.AlipayTradeNo, &o.Amount,
		&o.OriginalAmount, &o.Subject, &o.NotifyURL, &o.ReturnURL, &o.Token, &o.Status,
		&o.CreatedAt, &o.PaidAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}
