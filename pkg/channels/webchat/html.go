package webchat

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
  --bg-primary:#0f1117;--bg-secondary:#161822;--bg-input:#12141d;
  --border:#252836;--border-focus:#6c5ce7;
  --accent:#6c5ce7;--accent-hover:#5a4bd1;--accent-glow:rgba(108,92,231,.15);
  --text-primary:#e8e6f0;--text-secondary:#8b8a97;--text-muted:#5c5b66;
  --error:#f87171;--error-bg:rgba(248,113,113,.08);
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
  background:var(--bg-secondary);border:1px solid var(--border);border-radius:16px;
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
  font-family:inherit;outline:none;transition:border-color .2s,box-shadow .2s;
}
.field input::placeholder{color:var(--text-muted)}
.field input:focus{border-color:var(--border-focus);box-shadow:0 0 0 3px var(--accent-glow)}
.login-btn{
  width:100%;padding:12px;margin-top:8px;
  background:var(--accent);color:#fff;border:none;
  border-radius:10px;font-size:14px;font-weight:600;
  font-family:inherit;cursor:pointer;transition:background .2s,transform .1s;
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
