package vmq

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Client struct {
	baseURL   string
	key       string
	deviceKey string
	http      *http.Client
}

func NewClient(baseURL, key, deviceKey string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		key:       key,
		deviceKey: deviceKey,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

type CreateOrderResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	OrderID string `json:"orderId"`
	Amount  string `json:"payAmount"` // actual amount with tail digits
}

func (c *Client) CreateOrder(payType int, amount, orderID string) (*CreateOrderResponse, error) {
	params := url.Values{}
	params.Set("payId", orderID)
	params.Set("type", fmt.Sprintf("%d", payType))
	params.Set("price", amount)
	params.Set("sign", c.sign(params, c.key))

	resp, err := c.http.PostForm(c.baseURL+"/createOrder", params)
	if err != nil {
		return nil, fmt.Errorf("vmq createOrder: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vmq createOrder read body: %w", err)
	}

	var result CreateOrderResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmq createOrder parse: %w (body: %s)", err, string(body))
	}
	if result.Code != 1 {
		return nil, fmt.Errorf("vmq createOrder failed: %s", result.Msg)
	}
	return &result, nil
}

func (c *Client) AppPush(price string) error {
	params := url.Values{}
	params.Set("price", price)
	params.Set("type", "1") // alipay
	params.Set("sign", c.signPush(price))

	resp, err := c.http.PostForm(c.baseURL+"/appPush", params)
	if err != nil {
		return fmt.Errorf("vmq appPush: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("vmq appPush read body: %w", err)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("vmq appPush parse: %w (body: %s)", err, string(body))
	}
	if result.Code != 1 {
		return fmt.Errorf("vmq appPush failed: %s", result.Msg)
	}
	return nil
}

func (c *Client) sign(params url.Values, key string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "sign" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var buf strings.Builder
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(params.Get(k))
		buf.WriteString("&")
	}
	buf.WriteString(key)

	h := md5.Sum([]byte(buf.String()))
	return hex.EncodeToString(h[:])
}

func (c *Client) signPush(price string) string {
	raw := price + "1" + c.deviceKey
	h := md5.Sum([]byte(raw))
	return hex.EncodeToString(h[:])
}
