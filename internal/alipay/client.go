package alipay

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/smartwalle/alipay/v3"
)

type Client struct {
	client *alipay.Client
}

func NewClient(appID, privateKey, alipayPublicKey string) (*Client, error) {
	c, err := alipay.New(appID, privateKey, false)
	if err != nil {
		return nil, fmt.Errorf("init alipay client: %w", err)
	}
	if err := c.LoadAliPayPublicKey(alipayPublicKey); err != nil {
		return nil, fmt.Errorf("load alipay public key: %w", err)
	}
	return &Client{client: c}, nil
}

type PayRequest struct {
	OutTradeNo     string
	Subject        string
	TotalAmount    string
	NotifyURL      string
	ReturnURL      string
	TimeoutExpress string // e.g. "4m"
}

// PagePay returns an HTML form that auto-submits to Alipay (for PC).
func (c *Client) PagePay(req PayRequest) (string, error) {
	p := alipay.TradePagePay{
		Trade: alipay.Trade{
			Subject:        req.Subject,
			OutTradeNo:     req.OutTradeNo,
			TotalAmount:    req.TotalAmount,
			ProductCode:    "FAST_INSTANT_TRADE_PAY",
			NotifyURL:      req.NotifyURL,
			ReturnURL:      req.ReturnURL,
			TimeoutExpress: req.TimeoutExpress,
		},
	}
	result, err := c.client.TradePagePay(p)
	if err != nil {
		return "", fmt.Errorf("alipay page pay: %w", err)
	}
	return result.String(), nil
}

// WapPay returns a URL that redirects to Alipay H5 payment.
func (c *Client) WapPay(req PayRequest) (string, error) {
	p := alipay.TradeWapPay{
		Trade: alipay.Trade{
			Subject:        req.Subject,
			OutTradeNo:     req.OutTradeNo,
			TotalAmount:    req.TotalAmount,
			ProductCode:    "QUICK_WAP_WAY",
			NotifyURL:      req.NotifyURL,
			ReturnURL:      req.ReturnURL,
			TimeoutExpress: req.TimeoutExpress,
		},
	}
	result, err := c.client.TradeWapPay(p)
	if err != nil {
		return "", fmt.Errorf("alipay wap pay: %w", err)
	}
	return result.String(), nil
}

// PreCreate generates a QR code URL for face-to-face payment.
func (c *Client) PreCreate(ctx context.Context, req PayRequest) (string, error) {
	p := alipay.TradePreCreate{
		Trade: alipay.Trade{
			Subject:        req.Subject,
			OutTradeNo:     req.OutTradeNo,
			TotalAmount:    req.TotalAmount,
			NotifyURL:      req.NotifyURL,
			TimeoutExpress: req.TimeoutExpress,
		},
	}
	result, err := c.client.TradePreCreate(ctx, p)
	if err != nil {
		return "", fmt.Errorf("alipay precreate: %w", err)
	}
	if !result.IsSuccess() {
		return "", fmt.Errorf("alipay precreate failed: %s %s", result.SubCode, result.SubMsg)
	}
	return result.QRCode, nil
}

// Query checks the trade status.
func (c *Client) Query(ctx context.Context, outTradeNo string) (*alipay.TradeQueryRsp, error) {
	p := alipay.TradeQuery{
		OutTradeNo: outTradeNo,
	}
	result, err := c.client.TradeQuery(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("alipay query: %w", err)
	}
	return result, nil
}

// VerifyNotification verifies and parses an async notification from Alipay.
func (c *Client) VerifyNotification(req *http.Request) (*alipay.Notification, error) {
	if err := req.ParseForm(); err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}
	noti, err := c.client.DecodeNotification(req.Context(), req.Form)
	if err != nil {
		return nil, fmt.Errorf("verify notification: %w", err)
	}
	return noti, nil
}

// BuildPayURL builds the full Alipay gateway URL for redirect-based flows.
func (c *Client) BuildPayURL(values url.Values) string {
	return "https://openapi.alipay.com/gateway.do?" + values.Encode()
}
