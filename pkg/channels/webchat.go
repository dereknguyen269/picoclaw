package channels

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
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type WebChatChannel struct {
	*BaseChannel
	config   config.WebChatConfig
	server   *http.Server
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
	base := NewBaseChannel("webchat", cfg, msgBus, cfg.AllowFrom)
	return &WebChatChannel{
		BaseChannel: base,
		config:      cfg,
		messages:    make(map[string][]chatMessage),
		pending:     make(map[string]chan string),
		sessions:    make(map[string]time.Time),
	}, nil
}

// authEnabled returns true when both username and password are configured.
func (c *WebChatChannel) authEnabled() bool {
	return c.config.Username != "" && c.config.Password != ""
}

// createSession generates a random session token and stores it.
func (c *WebChatChannel) createSession() string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	c.mu.Lock()
	c.sessions[token] = time.Now().Add(24 * time.Hour)
	c.mu.Unlock()
	return token
}

// validSession checks if the request carries a valid session cookie.
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

// requireAuth wraps a handler with authentication. If auth is not configured, it passes through.
func (c *WebChatChannel) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.authEnabled() {
			next(w, r)
			return
		}
		if c.validSession(r) {
			next(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// requireAuthAPI is like requireAuth but returns 401 JSON for API endpoints.
func (c *WebChatChannel) requireAuthAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.authEnabled() {
			next(w, r)
			return
		}
		if c.validSession(r) {
			next(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}
}

func (c *WebChatChannel) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", c.requireAuth(c.handleUI))
	mux.HandleFunc("/chat/send", c.requireAuthAPI(c.handleSend))
	mux.HandleFunc("/chat/poll", c.requireAuthAPI(c.handlePoll))
	mux.HandleFunc("/login", c.handleLogin)
	mux.HandleFunc("/logout", c.handleLogout)

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	c.server = &http.Server{Addr: addr, Handler: mux}
	c.setRunning(true)

	if c.authEnabled() {
		logger.InfoCF("channels", "WebChat started (auth enabled)", map[string]interface{}{"addr": addr})
	} else {
		logger.InfoCF("channels", "WebChat started (no auth)", map[string]interface{}{"addr": addr})
	}

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.ErrorCF("channels", "WebChat server error", map[string]interface{}{"error": err.Error()})
		}
	}()

	return nil
}

func (c *WebChatChannel) Stop(ctx context.Context) error {
	c.setRunning(false)
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
	// If auth not configured, redirect to chat
	if !c.authEnabled() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Already logged in
	if c.validSession(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, webChatLoginHTML)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
			return
		}
	} else {
		r.ParseForm()
		body.Username = r.FormValue("username")
		body.Password = r.FormValue("password")
	}

	usernameMatch := subtle.ConstantTimeCompare([]byte(body.Username), []byte(c.config.Username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(body.Password), []byte(c.config.Password)) == 1

	if !usernameMatch || !passwordMatch {
		logger.WarnCF("channels", "WebChat login failed", map[string]interface{}{
			"remote": r.RemoteAddr,
		})
		if contentType == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, webChatLoginErrorHTML)
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

	if contentType == "application/json" {
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
		Name:     "picoclaw_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
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

	c.mu.Lock()
	c.messages[req.ChatID] = append(c.messages[req.ChatID], chatMessage{
		Role:    "user",
		Content: req.Message,
		Time:    time.Now().Format("15:04:05"),
	})
	respCh := make(chan string, 1)
	c.pending[req.ChatID] = respCh
	c.mu.Unlock()

	c.HandleMessage(senderID, req.ChatID, req.Message, nil, nil)

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

var webChatLoginHTML = webChatLoginPage("")

var webChatLoginErrorHTML = webChatLoginPage("Invalid username or password")

func webChatLoginPage(errMsg string) string {
	errBlock := ""
	if errMsg != "" {
		errBlock = `<div class="login-error"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="16" height="16"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>` + errMsg + `</div>`
	}
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>PicoClaw - Login</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
<style>
:root{
  --bg-primary:#0f1117;--bg-secondary:#161822;--bg-tertiary:#1c1f2e;
  --bg-input:#12141d;--border:#252836;--border-focus:#6c5ce7;
  --accent:#6c5ce7;--accent-hover:#5a4bd1;--accent-glow:rgba(108,92,231,.15);
  --text-primary:#e8e6f0;--text-secondary:#8b8a97;--text-muted:#5c5b66;
  --error:#f87171;--error-bg:rgba(248,113,113,.08);
  --radius:12px;
}
*{box-sizing:border-box;margin:0;padding:0}
html,body{height:100%}
body{
  font-family:'Inter',system-ui,-apple-system,sans-serif;
  background:var(--bg-primary);color:var(--text-primary);
  display:flex;align-items:center;justify-content:center;
  -webkit-font-smoothing:antialiased;
}
.login-card{
  width:100%;max-width:380px;padding:40px 32px;
  background:var(--bg-secondary);border:1px solid var(--border);
  border-radius:16px;
}
.login-logo{
  width:48px;height:48px;margin:0 auto 24px;
  background:linear-gradient(135deg,#6c5ce7,#a855f7);border-radius:14px;
  display:flex;align-items:center;justify-content:center;
}
.login-logo svg{width:24px;height:24px;color:#fff}
.login-card h1{font-size:20px;font-weight:600;text-align:center;margin-bottom:4px}
.login-card .sub{font-size:13px;color:var(--text-muted);text-align:center;margin-bottom:28px}
.login-error{
  display:flex;align-items:center;gap:8px;
  padding:10px 14px;margin-bottom:20px;
  background:var(--error-bg);border:1px solid rgba(248,113,113,.2);
  border-radius:8px;font-size:13px;color:var(--error);
}
.field{margin-bottom:16px}
.field label{display:block;font-size:13px;font-weight:500;color:var(--text-secondary);margin-bottom:6px}
.field input{
  width:100%;padding:11px 14px;
  background:var(--bg-input);border:1px solid var(--border);
  border-radius:8px;color:var(--text-primary);font-size:14px;
  font-family:inherit;outline:none;
  transition:border-color .2s,box-shadow .2s;
}
.field input::placeholder{color:var(--text-muted)}
.field input:focus{border-color:var(--border-focus);box-shadow:0 0 0 3px var(--accent-glow)}
.login-btn{
  width:100%;padding:12px;margin-top:8px;
  background:var(--accent);color:#fff;border:none;
  border-radius:10px;font-size:14px;font-weight:600;
  font-family:inherit;cursor:pointer;
  transition:background .2s,transform .1s;
}
.login-btn:hover{background:var(--accent-hover)}
.login-btn:active{transform:scale(.98)}
.login-btn:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
@media(max-width:440px){.login-card{margin:16px;padding:32px 24px}}
</style>
</head>
<body>
<form class="login-card" method="POST" action="/login">
  <div class="login-logo"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg></div>
  <h1>PicoClaw</h1>
  <p class="sub">Sign in to start chatting</p>
  ` + errBlock + `
  <div class="field"><label for="username">Username</label><input id="username" name="username" type="text" placeholder="Enter username" autocomplete="username" required autofocus></div>
  <div class="field"><label for="password">Password</label><input id="password" name="password" type="password" placeholder="Enter password" autocomplete="current-password" required></div>
  <button class="login-btn" type="submit">Sign in</button>
</form>
</body>
</html>`
}

var webChatHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>PicoClaw Chat</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
<style>
:root{
  --bg-primary:#0f1117;--bg-secondary:#161822;--bg-tertiary:#1c1f2e;
  --bg-input:#12141d;--border:#252836;--border-focus:#6c5ce7;
  --accent:#6c5ce7;--accent-hover:#5a4bd1;--accent-glow:rgba(108,92,231,.15);
  --text-primary:#e8e6f0;--text-secondary:#8b8a97;--text-muted:#5c5b66;
  --user-bg:linear-gradient(135deg,#6c5ce7 0%,#a855f7 100%);
  --assistant-bg:#1c1f2e;--code-bg:#0d0f18;
  --success:#34d399;--error:#f87171;
  --radius:12px;--radius-lg:16px;
}
*{box-sizing:border-box;margin:0;padding:0}
html,body{height:100%}
body{
  font-family:'Inter',system-ui,-apple-system,sans-serif;
  background:var(--bg-primary);color:var(--text-primary);
  display:flex;flex-direction:column;overflow:hidden;
  -webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale;
}
#header{
  padding:16px 24px;background:var(--bg-secondary);
  border-bottom:1px solid var(--border);
  display:flex;align-items:center;gap:12px;flex-shrink:0;
}
.logo-icon{
  width:36px;height:36px;background:var(--user-bg);border-radius:10px;
  display:flex;align-items:center;justify-content:center;flex-shrink:0;
}
.logo-icon svg{width:18px;height:18px;color:#fff}
#header .title-group{display:flex;flex-direction:column;gap:1px}
#header h1{font-size:16px;font-weight:600;color:var(--text-primary);letter-spacing:-.01em}
#header .subtitle{font-size:12px;color:var(--text-muted);font-weight:400}
.header-right{margin-left:auto;display:flex;align-items:center;gap:12px}
.status-dot{
  width:8px;height:8px;background:var(--success);border-radius:50%;
  box-shadow:0 0 8px rgba(52,211,153,.4);animation:pulse 2s ease-in-out infinite;
}
.logout-btn{
  background:none;border:1px solid var(--border);border-radius:8px;
  color:var(--text-secondary);padding:6px 12px;font-size:12px;
  font-family:inherit;cursor:pointer;display:flex;align-items:center;gap:6px;
  transition:border-color .2s,color .2s;
}
.logout-btn:hover{border-color:var(--text-muted);color:var(--text-primary)}
.logout-btn:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
.logout-btn svg{width:14px;height:14px}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.5}}
#messages{
  flex:1;overflow-y:auto;padding:24px;
  display:flex;flex-direction:column;gap:16px;
  scroll-behavior:smooth;scrollbar-width:thin;scrollbar-color:var(--border) transparent;
}
#messages::-webkit-scrollbar{width:6px}
#messages::-webkit-scrollbar-track{background:transparent}
#messages::-webkit-scrollbar-thumb{background:var(--border);border-radius:3px}
#messages::-webkit-scrollbar-thumb:hover{background:var(--text-muted)}
#empty-state{
  flex:1;display:flex;flex-direction:column;
  align-items:center;justify-content:center;
  gap:16px;text-align:center;padding:40px;
}
#empty-state svg{width:48px;height:48px;color:var(--text-muted);opacity:.5}
#empty-state p{font-size:14px;color:var(--text-muted);line-height:1.7;max-width:340px}
.msg-row{display:flex;gap:10px;animation:msgIn .3s ease-out}
.msg-row.user{flex-direction:row-reverse}
.msg-avatar{
  width:30px;height:30px;border-radius:9px;flex-shrink:0;
  display:flex;align-items:center;justify-content:center;margin-top:2px;
}
.msg-row.assistant .msg-avatar{background:var(--bg-tertiary);border:1px solid var(--border)}
.msg-row.user .msg-avatar{background:var(--user-bg)}
.msg-avatar svg{width:14px;height:14px;color:var(--text-secondary)}
.msg-row.user .msg-avatar svg{color:#fff}
.msg-bubble{
  max-width:72%;padding:12px 16px;border-radius:var(--radius-lg);
  line-height:1.65;font-size:14px;white-space:pre-wrap;word-wrap:break-word;
}
.msg-row.user .msg-bubble{background:var(--user-bg);color:#fff;border-bottom-right-radius:6px}
.msg-row.assistant .msg-bubble{
  background:var(--assistant-bg);color:var(--text-primary);
  border:1px solid var(--border);border-bottom-left-radius:6px;
}
.msg-bubble .time{font-size:11px;color:var(--text-muted);margin-top:6px;display:block}
.msg-row.user .msg-bubble .time{color:rgba(255,255,255,.45)}
.msg-bubble code{
  background:var(--code-bg);padding:2px 6px;border-radius:4px;
  font-size:13px;font-family:'SF Mono',SFMono-Regular,Consolas,monospace;
  border:1px solid var(--border);
}
.msg-bubble pre{
  background:var(--code-bg);padding:14px 16px;border-radius:8px;
  overflow-x:auto;margin:8px 0;border:1px solid var(--border);
}
.msg-bubble pre code{padding:0;background:none;border:none;font-size:13px;line-height:1.5}
.msg-bubble strong{font-weight:600;color:#c4b5fd}
#typing{
  padding:0 24px;min-height:28px;display:flex;align-items:center;gap:8px;flex-shrink:0;
}
.typing-dots{display:flex;gap:4px;align-items:center}
.typing-dots span{
  width:6px;height:6px;background:var(--accent);border-radius:50%;
  animation:bounce .6s infinite alternate;opacity:.5;
}
.typing-dots span:nth-child(2){animation-delay:.15s}
.typing-dots span:nth-child(3){animation-delay:.3s}
.typing-label{font-size:13px;color:var(--text-muted)}
#input-area{
  padding:16px 24px 20px;background:var(--bg-secondary);
  border-top:1px solid var(--border);flex-shrink:0;
}
.input-wrapper{
  display:flex;align-items:flex-end;gap:10px;
  background:var(--bg-input);border:1px solid var(--border);
  border-radius:var(--radius);padding:4px 4px 4px 16px;
  transition:border-color .2s ease,box-shadow .2s ease;
}
.input-wrapper:focus-within{border-color:var(--border-focus);box-shadow:0 0 0 3px var(--accent-glow)}
#input{
  flex:1;padding:10px 0;border:none;font-size:14px;
  background:transparent;color:var(--text-primary);
  outline:none;resize:none;max-height:120px;font-family:inherit;line-height:1.5;
}
#input::placeholder{color:var(--text-muted)}
#send{
  width:40px;height:40px;background:var(--accent);color:#fff;
  border:none;border-radius:10px;cursor:pointer;
  display:flex;align-items:center;justify-content:center;flex-shrink:0;
  transition:background .2s ease,transform .1s ease,opacity .2s ease;
}
#send:hover{background:var(--accent-hover)}
#send:active{transform:scale(.95)}
#send:disabled{opacity:.35;cursor:not-allowed;transform:none}
#send:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
#send svg{width:18px;height:18px}
.hint{font-size:11px;color:var(--text-muted);text-align:center;margin-top:8px}
@keyframes msgIn{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:translateY(0)}}
@keyframes bounce{from{transform:translateY(0)}to{transform:translateY(-4px);opacity:1}}
@media(prefers-reduced-motion:reduce){
  .msg-row{animation:none}.typing-dots span{animation:none;opacity:.8}.status-dot{animation:none}
}
@media(max-width:640px){
  #messages{padding:16px}#input-area{padding:12px 16px 16px}
  .msg-bubble{max-width:85%;font-size:15px}#header{padding:14px 16px}
}
</style>
</head>
<body>
<div id="header">
  <div class="logo-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg></div>
  <div class="title-group"><h1>PicoClaw</h1><span class="subtitle">AI Assistant</span></div>
  <div class="header-right">
    <div class="status-dot" title="Online"></div>
    <a href="/logout" class="logout-btn" aria-label="Sign out"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>Sign out</a>
  </div>
</div>
<div id="messages">
  <div id="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>
    <p>Start a conversation with PicoClaw. Ask anything and get helpful responses.</p>
  </div>
</div>
<div id="typing"></div>
<div id="input-area">
  <div class="input-wrapper">
    <textarea id="input" rows="1" placeholder="Message PicoClaw..." aria-label="Chat message input"></textarea>
    <button id="send" aria-label="Send message"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg></button>
  </div>
  <div class="hint">Press Enter to send · Shift+Enter for new line</div>
</div>
<script>
const msgsEl=document.getElementById("messages"),
      input=document.getElementById("input"),
      btn=document.getElementById("send"),
      typingEl=document.getElementById("typing"),
      emptyState=document.getElementById("empty-state");
let busy=false;
function esc(s){return s.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;")}
function renderContent(raw){
  let t=esc(raw);
  t=t.replace(/` + "```" + `(\\w*)\\n([\\s\\S]*?)` + "```" + `/g,function(_,lang,code){return '<pre><code>'+code.trim()+'</code></pre>'});
  t=t.replace(/` + "`([^`]+)`" + `/g,'<code>$1</code>');
  t=t.replace(/\*\*(.+?)\*\*/g,'<strong>$1</strong>');
  return t;
}
function addMsg(role,content,time){
  if(emptyState&&emptyState.parentNode)emptyState.remove();
  const row=document.createElement("div");row.className="msg-row "+role;
  const av=document.createElement("div");av.className="msg-avatar";
  if(role==="user"){
    av.innerHTML='<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>';
  }else{
    av.innerHTML='<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>';
  }
  const bubble=document.createElement("div");bubble.className="msg-bubble";
  bubble.innerHTML=renderContent(content)+(time?'<span class="time">'+time+'</span>':'');
  row.appendChild(av);row.appendChild(bubble);
  msgsEl.appendChild(row);msgsEl.scrollTop=msgsEl.scrollHeight;
}
function showTyping(){typingEl.innerHTML='<div class="typing-dots"><span></span><span></span><span></span></div><span class="typing-label">PicoClaw is thinking...</span>'}
function hideTyping(){typingEl.innerHTML=''}
async function send(){
  const m=input.value.trim();if(!m||busy)return;
  busy=true;btn.disabled=true;input.value="";input.style.height="auto";
  const ts=new Date().toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'});
  addMsg("user",m,ts);showTyping();
  try{
    const r=await fetch("/chat/send",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({message:m,chat_id:"default"})});
    if(r.status===401){window.location.href="/login";return}
    if(!r.ok)throw new Error(r.statusText);
    const d=await r.json();
    addMsg("assistant",d.message,new Date().toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'}));
  }catch(e){addMsg("assistant","Something went wrong: "+e.message,"")}
  hideTyping();busy=false;btn.disabled=false;input.focus();
}
btn.onclick=send;
input.onkeydown=e=>{if(e.key==="Enter"&&!e.shiftKey){e.preventDefault();send()}};
input.oninput=()=>{input.style.height="auto";input.style.height=Math.min(input.scrollHeight,120)+"px"};
input.focus();
</script>
</body>
</html>`
