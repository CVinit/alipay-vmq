package handler

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alipay-vmq/internal/alipay"
)

//go:embed templates
var templateFS embed.FS

var payTemplate = template.Must(template.ParseFS(templateFS, "templates/pay.html"))

type payPageData struct {
	OrderID        string
	Token          string
	Amount         string
	Subject        string
	QRCodeURL      string
	WapPayURL      string
	PagePayURL     string
	StatusURL      string
	ReturnURL      string
	IsMobile       bool
	Expired        bool
	RemainingSeconds int
}

func (s *Server) handlePayPage(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	token := r.URL.Query().Get("token")
	if orderID == "" || token == "" {
		http.Error(w, "missing params", http.StatusBadRequest)
		return
	}

	order, err := s.store.GetOrder(context.Background(), orderID)
	if err != nil || order.Token != token {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	if order.Status != "pending" {
		data := &payPageData{
			OrderID: order.ID,
			Amount:  order.Amount,
			Subject: order.Subject,
			Expired: order.Status == "expired",
		}
		w.Header().Set("Cache-Control", "no-store")
		payTemplate.Execute(w, data)
		return
	}

	isMobile := detectMobile(r.UserAgent())
	notifyURL := fmt.Sprintf("%s/notify", s.cfg.PublicBaseURL)
	returnURL := fmt.Sprintf("%s/return?out_trade_no=%s", s.cfg.PublicBaseURL, order.ID)
	timeoutMinutes := s.cfg.VMQOrderTimeout - 1
	if timeoutMinutes < 1 {
		timeoutMinutes = 1
	}

	payReq := alipay.PayRequest{
		OutTradeNo:     order.ID,
		Subject:        order.Subject,
		TotalAmount:    formatAmount(order.Amount),
		NotifyURL:      notifyURL,
		ReturnURL:      returnURL,
		TimeoutExpress: fmt.Sprintf("%dm", timeoutMinutes),
	}

	data := &payPageData{
		OrderID:          order.ID,
		Token:            order.Token,
		Amount:           order.Amount,
		Subject:          order.Subject,
		StatusURL:        fmt.Sprintf("%s/api/order/status?order_id=%s&token=%s", s.cfg.PublicBaseURL, order.ID, order.Token),
		ReturnURL:        order.ReturnURL,
		IsMobile:         isMobile,
		RemainingSeconds: remainingSeconds(order.CreatedAt, s.cfg.VMQOrderTimeout),
	}

	if s.cfg.AlipayPreCreate {
		qrURL, err := s.alipay.PreCreate(context.Background(), payReq)
		if err != nil {
			slog.Error("precreate failed", "order_id", order.ID, "error", err)
		} else {
			data.QRCodeURL = qrURL
		}
	}

	if isMobile {
		wapURL, err := s.alipay.WapPay(payReq)
		if err != nil {
			slog.Error("wap pay failed", "order_id", order.ID, "error", err)
		} else {
			data.WapPayURL = wapURL
		}
	}

	pageURL, err := s.alipay.PagePay(payReq)
	if err != nil {
		slog.Error("page pay failed", "order_id", order.ID, "error", err)
	} else {
		data.PagePayURL = pageURL
	}

	w.Header().Set("Cache-Control", "no-store")
	payTemplate.Execute(w, data)
}

func detectMobile(ua string) bool {
	ua = strings.ToLower(ua)
	mobileKeywords := []string{"mobile", "android", "iphone", "ipad", "ipod", "webos", "opera mini", "ucbrowser"}
	for _, kw := range mobileKeywords {
		if strings.Contains(ua, kw) {
			return true
		}
	}
	return false
}

// formatAmount ensures the amount has exactly 2 decimal places for Alipay.
func formatAmount(amount string) string {
	f, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return amount
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func remainingSeconds(createdAt time.Time, timeoutMinutes int) int {
	deadline := createdAt.Add(time.Duration(timeoutMinutes) * time.Minute)
	remaining := int(time.Until(deadline).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}
