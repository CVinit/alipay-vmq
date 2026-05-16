package vmq

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
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

type createOrderRaw struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		OrderID     string  `json:"orderId"`
		ReallyPrice float64 `json:"reallyPrice"`
		PayURL      string  `json:"payUrl"`
	} `json:"data"`
}

type CreateOrderResponse struct {
	OrderID string
	Amount  string
}

func (c *Client) CreateOrder(payType int, amount, orderID string) (*CreateOrderResponse, error) {
	payTypeStr := strconv.Itoa(payType)
	// VMQ sign = md5(payId + param + type + price + key)
	sign := md5Hex(orderID + "" + payTypeStr + amount + c.key)

	params := url.Values{}
	params.Set("payId", orderID)
	params.Set("type", payTypeStr)
	params.Set("price", amount)
	params.Set("sign", sign)

	resp, err := c.http.PostForm(c.baseURL+"/createOrder", params)
	if err != nil {
		return nil, fmt.Errorf("vmq createOrder: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vmq createOrder read body: %w", err)
	}

	slog.Debug("vmq createOrder response", "body", string(body))

	var raw createOrderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("vmq createOrder parse: %w (body: %s)", err, string(body))
	}
	if raw.Code != 1 {
		return nil, fmt.Errorf("vmq createOrder failed: %s", raw.Msg)
	}

	result := &CreateOrderResponse{
		OrderID: raw.Data.OrderID,
		Amount:  strconv.FormatFloat(raw.Data.ReallyPrice, 'f', -1, 64),
	}
	slog.Info("vmq createOrder success", "orderId", result.OrderID, "reallyPrice", result.Amount)
	return result, nil
}

func (c *Client) AppPush(price string) error {
	// VMQ sign = md5(type + price + t + deviceKey)
	t := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sign := md5Hex("2" + price + t + c.deviceKey)

	params := url.Values{}
	params.Set("price", price)
	params.Set("type", "2")
	params.Set("t", t)
	params.Set("sign", sign)

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

func md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
