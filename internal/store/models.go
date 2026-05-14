package store

import "time"

type OrderStatus string

const (
	StatusPending OrderStatus = "pending"
	StatusPaid    OrderStatus = "paid"
	StatusExpired OrderStatus = "expired"
)

type Order struct {
	ID             string
	EpayTradeNo    string // Dujiao-Next's out_trade_no
	VMQOrderID     string
	AlipayTradeNo  string
	Amount         string // VMQ-assigned amount with tail digits
	OriginalAmount string // original amount from merchant
	Subject        string
	NotifyURL      string
	ReturnURL      string
	Token          string // for pay page access control
	Status         OrderStatus
	CreatedAt      time.Time
	PaidAt         *time.Time
}

type Notification struct {
	ID            int64
	AlipayTradeNo string
	OrderID       string
	RawBody       string
	ReceivedAt    time.Time
}
