package epay

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

type Handler struct {
	merchantID  string
	merchantKey string
	onCreate    func(req *CreateRequest) (*CreateResponse, error)
}

type CreateRequest struct {
	PID        string
	Type       string // "alipay"
	OutTradeNo string
	NotifyURL  string
	ReturnURL  string
	Name       string
	Money      string
	ClientIP   string
}

type CreateResponse struct {
	PayURL string
}

func NewHandler(merchantID, merchantKey string, onCreate func(*CreateRequest) (*CreateResponse, error)) *Handler {
	return &Handler{
		merchantID:  merchantID,
		merchantKey: merchantKey,
		onCreate:    onCreate,
	}
}

// HandleMapi handles POST /mapi.php (epay v1 JSON API).
func (h *Handler) HandleMapi(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEpayError(w, "method not allowed")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeEpayError(w, "invalid form data")
		return
	}

	req, err := h.validateAndParse(r.PostForm)
	if err != nil {
		writeEpayError(w, err.Error())
		return
	}
	req.ClientIP = clientIP(r)

	resp, err := h.onCreate(req)
	if err != nil {
		slog.Error("epay create order failed", "error", err)
		writeEpayError(w, "create order failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"code":   1,
		"msg":    "success",
		"payurl": resp.PayURL,
	})
}

// HandleSubmit handles GET|POST /submit.php (redirect-based flow).
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	params := r.Form
	req, err := h.validateAndParse(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ClientIP = clientIP(r)

	resp, err := h.onCreate(req)
	if err != nil {
		slog.Error("epay submit create order failed", "error", err)
		http.Error(w, "create order failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, resp.PayURL, http.StatusFound)
}

func (h *Handler) validateAndParse(params url.Values) (*CreateRequest, error) {
	pid := params.Get("pid")
	if pid != h.merchantID {
		return nil, fmt.Errorf("invalid pid")
	}

	signType := params.Get("sign_type")
	if signType != "" && signType != "MD5" {
		return nil, fmt.Errorf("unsupported sign_type: %s", signType)
	}

	if !Verify(params, h.merchantKey) {
		return nil, fmt.Errorf("invalid sign")
	}

	outTradeNo := params.Get("out_trade_no")
	if outTradeNo == "" {
		return nil, fmt.Errorf("out_trade_no is required")
	}

	money := params.Get("money")
	if money == "" {
		return nil, fmt.Errorf("money is required")
	}

	payType := params.Get("type")
	if payType != "alipay" {
		return nil, fmt.Errorf("unsupported type: %s (only alipay supported)", payType)
	}

	return &CreateRequest{
		PID:        pid,
		Type:       payType,
		OutTradeNo: outTradeNo,
		NotifyURL:  params.Get("notify_url"),
		ReturnURL:  params.Get("return_url"),
		Name:       params.Get("name"),
		Money:      money,
	}, nil
}

func writeEpayError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]any{
		"code": 0,
		"msg":  msg,
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}
