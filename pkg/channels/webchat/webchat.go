package webchat

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// WebChatChannel is a self-contained HTTP chat channel with a built-in web UI.
// It runs its own HTTP server on a dedicated port, independent of the shared gateway.
type WebChatChannel struct {
	*channels.BaseChannel
	config   config.WebChatConfig
	server   *http.Server
	ctx      context.Context
	cancel   context.CancelFunc
	messages map[string][]chatMessage // chatID -> messages
	pending  map[string]chan string   // chatID -> response channel
	sessions map[string]time.Time     // token -> expiry
	mu       sync.RWMutex
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    string `json:"time"`
}

type chatRequest struct {
	ChatID  string `json:"chat_id"`
	Message string `json:"message"`
}

type chatResponse struct {
	ChatID  string `json:"chat_id"`
	Message string `json:"message"`
}

func NewWebChatChannel(cfg config.WebChatConfig, msgBus *bus.MessageBus) (*WebChatChannel, error) {
	base := channels.NewBaseChannel("webchat", cfg, msgBus, cfg.AllowFrom)
	return &WebChatChannel{
		BaseChannel: base,
		config:      cfg,
		messages:    make(map[string][]chatMessage),
		pending:     make(map[string]chan string),
		sessions:    make(map[string]time.Time),
	}, nil
}

func (c *WebChatChannel) authEnabled() bool {
	return c.config.Username != "" && c.config.Password != ""
}

func (c *WebChatChannel) createSession() string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	c.mu.Lock()
	c.sessions[token] = time.Now().Add(24 * time.Hour)
	c.mu.Unlock()
	return token
}

func (c *WebChatChannel) validSession(r *http.Request) bool {
	cookie, err := r.Cookie("picoclaw_session")
	if err != nil {
		return false
	}
	c.mu.RLock()
	expiry, ok := c.sessions[cookie.Value]
	c.mu.RUnlock()
	return ok && time.Now().Before(expiry)
}

func (c *WebChatChannel) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.authEnabled() || c.validSession(r) {
			next(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func (c *WebChatChannel) requireAuthAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.authEnabled() || c.validSession(r) {
			next(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}
}

func (c *WebChatChannel) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", c.requireAuth(c.handleUI))
	mux.HandleFunc("/chat/send", c.requireAuthAPI(c.handleSend))
	mux.HandleFunc("/chat/poll", c.requireAuthAPI(c.handlePoll))
	mux.HandleFunc("/login", c.handleLogin)
	mux.HandleFunc("/logout", c.handleLogout)

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	c.server = &http.Server{Addr: addr, Handler: mux}
	c.SetRunning(true)

	if c.authEnabled() {
		logger.InfoCF("webchat", "WebChat started (auth enabled)", map[string]any{"addr": addr})
	} else {
		logger.InfoCF("webchat", "WebChat started (no auth)", map[string]any{"addr": addr})
	}

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.ErrorCF("webchat", "WebChat server error", map[string]any{"error": err.Error()})
		}
	}()

	return nil
}

func (c *WebChatChannel) Stop(ctx context.Context) error {
	c.SetRunning(false)
	if c.cancel != nil {
		c.cancel()
	}
	if c.server != nil {
		return c.server.Shutdown(ctx)
	}
	return nil
}

func (c *WebChatChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	c.mu.Lock()
	c.messages[msg.ChatID] = append(c.messages[msg.ChatID], chatMessage{
		Role:    "assistant",
		Content: msg.Content,
		Time:    time.Now().Format("15:04:05"),
	})
	ch, ok := c.pending[msg.ChatID]
	if ok {
		delete(c.pending, msg.ChatID)
	}
	c.mu.Unlock()

	if ok {
		select {
		case ch <- msg.Content:
		default:
		}
	}
	return nil
}

func (c *WebChatChannel) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !c.authEnabled() || c.validSession(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, webChatLoginPage(""))
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var username, password string
	ct := r.Header.Get("Content-Type")
	if ct == "application/json" {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		username, password = body.Username, body.Password
	} else {
		r.ParseForm()
		username = r.FormValue("username")
		password = r.FormValue("password")
	}

	usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(c.config.Username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(c.config.Password)) == 1

	if !usernameOK || !passwordOK {
		logger.WarnCF("webchat", "Login failed", map[string]any{"remote": r.RemoteAddr})
		if ct == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, webChatLoginPage("Invalid username or password"))
		return
	}

	token := c.createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     "picoclaw_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})

	if ct == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *WebChatChannel) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("picoclaw_session"); err == nil {
		c.mu.Lock()
		delete(c.sessions, cookie.Value)
		c.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name: "picoclaw_session", Value: "", Path: "/",
		HttpOnly: true, MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (c *WebChatChannel) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.ChatID == "" {
		req.ChatID = "default"
	}

	senderID := r.RemoteAddr
	peer := bus.Peer{Kind: "direct", ID: "webchat:" + req.ChatID}
	sender := bus.SenderInfo{
		Platform:    "webchat",
		PlatformID:  senderID,
		CanonicalID: identity.BuildCanonicalID("webchat", senderID),
	}

	c.mu.Lock()
	c.messages[req.ChatID] = append(c.messages[req.ChatID], chatMessage{
		Role: "user", Content: req.Message, Time: time.Now().Format("15:04:05"),
	})
	respCh := make(chan string, 1)
	c.pending[req.ChatID] = respCh
	c.mu.Unlock()

	msgID := fmt.Sprintf("wc-%d", time.Now().UnixNano())
	c.HandleMessage(c.ctx, peer, msgID, senderID, req.ChatID, req.Message, nil, nil, sender)

	select {
	case reply := <-respCh:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse{ChatID: req.ChatID, Message: reply})
	case <-time.After(120 * time.Second):
		http.Error(w, "timeout waiting for response", http.StatusGatewayTimeout)
	case <-r.Context().Done():
		return
	}
}

func (c *WebChatChannel) handlePoll(w http.ResponseWriter, r *http.Request) {
	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		chatID = "default"
	}
	c.mu.RLock()
	msgs := c.messages[chatID]
	c.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func (c *WebChatChannel) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, webChatHTML)
}
