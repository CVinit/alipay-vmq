package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"alipay-vmq/internal/alipay"
	"alipay-vmq/internal/config"
	"alipay-vmq/internal/epay"
	"alipay-vmq/internal/store"
	"alipay-vmq/internal/vmq"
)

type Server struct {
	cfg    *config.Config
	store  store.Store
	alipay *alipay.Client
	vmq    *vmq.Client
	epay   *epay.Handler
	mux    *http.ServeMux
}

func NewServer(cfg *config.Config, st store.Store, ac *alipay.Client, vc *vmq.Client) *Server {
	s := &Server{
		cfg:    cfg,
		store:  st,
		alipay: ac,
		vmq:    vc,
		mux:    http.NewServeMux(),
	}

	s.epay = epay.NewHandler(cfg.EpayMerchantID, cfg.EpayMerchantKey, s.handleCreateOrder)

	s.mux.HandleFunc("POST /mapi.php", s.epay.HandleMapi)
	s.mux.HandleFunc("GET /submit.php", s.epay.HandleSubmit)
	s.mux.HandleFunc("POST /submit.php", s.epay.HandleSubmit)
	s.mux.HandleFunc("GET /pay", s.handlePayPage)
	s.mux.HandleFunc("POST /notify", s.handleNotify)
	s.mux.HandleFunc("GET /return", s.handleReturn)
	s.mux.HandleFunc("GET /api/order/status", s.handleOrderStatus)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleCreateOrder(req *epay.CreateRequest) (*epay.CreateResponse, error) {
	existing, _ := s.store.GetOrderByEpayTradeNo(context.Background(), req.OutTradeNo)
	if existing != nil && existing.Status == store.StatusPending {
		payURL := fmt.Sprintf("%s/pay?order_id=%s&token=%s", s.cfg.PublicBaseURL, existing.ID, existing.Token)
		return &epay.CreateResponse{PayURL: payURL}, nil
	}

	orderID := generateID()
	token := generateToken()

	vmqResp, err := s.vmq.CreateOrder(2, req.Money, orderID, "")
	if err != nil {
		return nil, fmt.Errorf("vmq create order: %w", err)
	}

	order := &store.Order{
		ID:             orderID,
		EpayTradeNo:    req.OutTradeNo,
		VMQOrderID:     vmqResp.OrderID,
		Amount:         vmqResp.Amount,
		OriginalAmount: req.Money,
		Subject:        req.Name,
		NotifyURL:      req.NotifyURL,
		ReturnURL:      req.ReturnURL,
		Token:          token,
		Status:         store.StatusPending,
		CreatedAt:      time.Now(),
	}

	if err := s.store.CreateOrder(context.Background(), order); err != nil {
		return nil, fmt.Errorf("store create order: %w", err)
	}

	payURL := fmt.Sprintf("%s/pay?order_id=%s&token=%s", s.cfg.PublicBaseURL, orderID, token)
	return &epay.CreateResponse{PayURL: payURL}, nil
}

func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	noti, err := s.alipay.VerifyNotification(r)
	if err != nil {
		slog.Error("notify verify failed", "error", err)
		http.Error(w, "verify failed", http.StatusBadRequest)
		return
	}

	if noti.TradeStatus != "TRADE_SUCCESS" && noti.TradeStatus != "TRADE_FINISHED" {
		fmt.Fprint(w, "success")
		return
	}

	outTradeNo := noti.OutTradeNo
	order, err := s.store.GetOrder(context.Background(), outTradeNo)
	if err != nil {
		slog.Error("notify order not found", "out_trade_no", outTradeNo, "error", err)
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	if order.Status != store.StatusPending {
		fmt.Fprint(w, "success")
		return
	}

	already, _ := s.store.HasNotification(context.Background(), noti.TradeNo)
	if already {
		fmt.Fprint(w, "success")
		return
	}

	s.store.SaveNotification(context.Background(), &store.Notification{
		AlipayTradeNo: noti.TradeNo,
		OrderID:       order.ID,
		RawBody:       r.PostForm.Encode(),
		ReceivedAt:    time.Now(),
	})

	s.store.UpdateAlipayTradeNo(context.Background(), order.ID, noti.TradeNo)

	if err := s.vmq.AppPush(order.Amount); err != nil {
		slog.Error("notify appPush failed", "order_id", order.ID, "error", err)
	}

	if err := s.store.MarkPaid(context.Background(), order.ID); err != nil {
		slog.Error("notify mark paid failed", "order_id", order.ID, "error", err)
	}

	s.notifyMerchant(order, noti.TradeNo)

	fmt.Fprint(w, "success")
}

func (s *Server) handleReturn(w http.ResponseWriter, r *http.Request) {
	outTradeNo := r.URL.Query().Get("out_trade_no")
	if outTradeNo == "" {
		http.Error(w, "missing out_trade_no", http.StatusBadRequest)
		return
	}

	order, err := s.store.GetOrder(context.Background(), outTradeNo)
	if err != nil {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	if order.Status == store.StatusPending {
		s.tryQueryAndPush(context.Background(), order)
	}

	if order.ReturnURL != "" {
		sep := "?"
		if strings.Contains(order.ReturnURL, "?") {
			sep = "&"
		}
		redirectURL := fmt.Sprintf("%s%sout_trade_no=%s&trade_no=%s&type=alipay&pid=%s",
			order.ReturnURL, sep, order.EpayTradeNo, order.AlipayTradeNo, s.cfg.EpayMerchantID)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	fmt.Fprint(w, "payment completed")
}

func (s *Server) handleOrderStatus(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	token := r.URL.Query().Get("token")
	if orderID == "" || token == "" {
		http.Error(w, "missing params", http.StatusBadRequest)
		return
	}

	order, err := s.store.GetOrder(context.Background(), orderID)
	if err != nil || order.Token != token {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `{"status":"%s"}`, order.Status)
}

func (s *Server) TryQueryAndPush(ctx context.Context, order *store.Order) {
	s.tryQueryAndPush(ctx, order)
}

func (s *Server) tryQueryAndPush(ctx context.Context, order *store.Order) {
	result, err := s.alipay.Query(ctx, order.ID)
	if err != nil {
		slog.Debug("query alipay failed", "order_id", order.ID, "error", err)
		return
	}

	if result.TradeStatus != "TRADE_SUCCESS" && result.TradeStatus != "TRADE_FINISHED" {
		return
	}

	already, _ := s.store.HasNotification(ctx, result.TradeNo)
	if already {
		return
	}

	s.store.SaveNotification(ctx, &store.Notification{
		AlipayTradeNo: result.TradeNo,
		OrderID:       order.ID,
		RawBody:       fmt.Sprintf("query_result:trade_no=%s,status=%s", result.TradeNo, result.TradeStatus),
		ReceivedAt:    time.Now(),
	})
	s.store.UpdateAlipayTradeNo(ctx, order.ID, result.TradeNo)

	if err := s.vmq.AppPush(order.Amount); err != nil {
		slog.Error("poll appPush failed", "order_id", order.ID, "error", err)
		return
	}

	s.store.MarkPaid(ctx, order.ID)
	s.notifyMerchant(order, result.TradeNo)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) notifyMerchant(order *store.Order, tradeNo string) {
	if order.NotifyURL == "" {
		return
	}

	params := url.Values{}
	params.Set("pid", s.cfg.EpayMerchantID)
	params.Set("trade_no", tradeNo)
	params.Set("out_trade_no", order.EpayTradeNo)
	params.Set("type", "alipay")
	params.Set("name", order.Subject)
	params.Set("money", order.OriginalAmount)
	params.Set("trade_status", "TRADE_SUCCESS")
	params.Set("sign", epay.Sign(params, s.cfg.EpayMerchantKey))
	params.Set("sign_type", "MD5")

	resp, err := http.PostForm(order.NotifyURL, params)
	if err != nil {
		slog.Error("notify merchant failed", "order_id", order.ID, "url", order.NotifyURL, "error", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	slog.Info("notify merchant", "order_id", order.ID, "status", resp.StatusCode, "body", string(body))
}
