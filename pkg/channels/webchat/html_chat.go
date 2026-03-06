package webchat

// webChatHTML is the complete chat page served at "/".
var webChatHTML = chatHTMLHead + chatHTMLBody + chatHTMLScript + `</body></html>`

var chatHTMLHead = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>PicoClaw Chat</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
<style>
:root{--bg-primary:#0f1117;--bg-secondary:#161822;--bg-tertiary:#1c1f2e;--bg-input:#12141d;--border:#252836;--border-focus:#6c5ce7;--accent:#6c5ce7;--accent-hover:#5a4bd1;--accent-glow:rgba(108,92,231,.15);--text-primary:#e8e6f0;--text-secondary:#8b8a97;--text-muted:#5c5b66;--user-bg:linear-gradient(135deg,#6c5ce7 0%,#a855f7 100%);--assistant-bg:#1c1f2e;--code-bg:#0d0f18;--success:#34d399;--radius:12px;--radius-lg:16px}
*{box-sizing:border-box;margin:0;padding:0}html,body{height:100%}
body{font-family:'Inter',system-ui,-apple-system,sans-serif;background:var(--bg-primary);color:var(--text-primary);display:flex;flex-direction:column;overflow:hidden;-webkit-font-smoothing:antialiased}
#header{padding:16px 24px;background:var(--bg-secondary);border-bottom:1px solid var(--border);display:flex;align-items:center;gap:12px;flex-shrink:0}
.logo-icon{width:36px;height:36px;background:var(--user-bg);border-radius:10px;display:flex;align-items:center;justify-content:center;flex-shrink:0}
.logo-icon svg{width:18px;height:18px;color:#fff}
#header .title-group{display:flex;flex-direction:column;gap:1px}
#header h1{font-size:16px;font-weight:600;color:var(--text-primary);letter-spacing:-.01em}
#header .subtitle{font-size:12px;color:var(--text-muted);font-weight:400}
.header-right{margin-left:auto;display:flex;align-items:center;gap:12px}
.status-dot{width:8px;height:8px;background:var(--success);border-radius:50%;box-shadow:0 0 8px rgba(52,211,153,.4);animation:pulse 2s ease-in-out infinite}
.logout-btn{background:none;border:1px solid var(--border);border-radius:8px;color:var(--text-secondary);padding:6px 12px;font-size:12px;font-family:inherit;cursor:pointer;display:flex;align-items:center;gap:6px;transition:border-color .2s,color .2s;text-decoration:none}
.logout-btn:hover{border-color:var(--text-muted);color:var(--text-primary)}
.logout-btn:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
.logout-btn svg{width:14px;height:14px}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.5}}
#messages{flex:1;overflow-y:auto;padding:24px;display:flex;flex-direction:column;gap:16px;scroll-behavior:smooth;scrollbar-width:thin;scrollbar-color:var(--border) transparent}
#messages::-webkit-scrollbar{width:6px}#messages::-webkit-scrollbar-track{background:transparent}#messages::-webkit-scrollbar-thumb{background:var(--border);border-radius:3px}
#empty-state{flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:16px;text-align:center;padding:40px}
#empty-state svg{width:48px;height:48px;color:var(--text-muted);opacity:.5}
#empty-state p{font-size:14px;color:var(--text-muted);line-height:1.7;max-width:340px}
.msg-row{display:flex;gap:10px;animation:msgIn .3s ease-out}.msg-row.user{flex-direction:row-reverse}
.msg-avatar{width:30px;height:30px;border-radius:9px;flex-shrink:0;display:flex;align-items:center;justify-content:center;margin-top:2px}
.msg-row.assistant .msg-avatar{background:var(--bg-tertiary);border:1px solid var(--border)}
.msg-row.user .msg-avatar{background:var(--user-bg)}
.msg-avatar svg{width:14px;height:14px;color:var(--text-secondary)}.msg-row.user .msg-avatar svg{color:#fff}
.msg-bubble{max-width:72%;padding:12px 16px;border-radius:var(--radius-lg);line-height:1.65;font-size:14px;white-space:pre-wrap;word-wrap:break-word}
.msg-row.user .msg-bubble{background:var(--user-bg);color:#fff;border-bottom-right-radius:6px}
.msg-row.assistant .msg-bubble{background:var(--assistant-bg);color:var(--text-primary);border:1px solid var(--border);border-bottom-left-radius:6px}
.msg-bubble .time{font-size:11px;color:var(--text-muted);margin-top:6px;display:block}
.msg-row.user .msg-bubble .time{color:rgba(255,255,255,.45)}
.msg-bubble code{background:var(--code-bg);padding:2px 6px;border-radius:4px;font-size:13px;font-family:'SF Mono',SFMono-Regular,Consolas,monospace;border:1px solid var(--border)}
.msg-bubble pre{background:var(--code-bg);padding:14px 16px;border-radius:8px;overflow-x:auto;margin:8px 0;border:1px solid var(--border)}
.msg-bubble pre code{padding:0;background:none;border:none;font-size:13px;line-height:1.5}
.msg-bubble strong{font-weight:600;color:#c4b5fd}
#typing{padding:0 24px;min-height:28px;display:flex;align-items:center;gap:8px;flex-shrink:0}
.typing-dots{display:flex;gap:4px;align-items:center}
.typing-dots span{width:6px;height:6px;background:var(--accent);border-radius:50%;animation:bounce .6s infinite alternate;opacity:.5}
.typing-dots span:nth-child(2){animation-delay:.15s}.typing-dots span:nth-child(3){animation-delay:.3s}
.typing-label{font-size:13px;color:var(--text-muted)}
#input-area{padding:16px 24px 20px;background:var(--bg-secondary);border-top:1px solid var(--border);flex-shrink:0}
.input-wrapper{display:flex;align-items:flex-end;gap:10px;background:var(--bg-input);border:1px solid var(--border);border-radius:var(--radius);padding:4px 4px 4px 16px;transition:border-color .2s ease,box-shadow .2s ease}
.input-wrapper:focus-within{border-color:var(--border-focus);box-shadow:0 0 0 3px var(--accent-glow)}
#input{flex:1;padding:10px 0;border:none;font-size:14px;background:transparent;color:var(--text-primary);outline:none;resize:none;max-height:120px;font-family:inherit;line-height:1.5}
#input::placeholder{color:var(--text-muted)}
#send{width:40px;height:40px;background:var(--accent);color:#fff;border:none;border-radius:10px;cursor:pointer;display:flex;align-items:center;justify-content:center;flex-shrink:0;transition:background .2s ease,transform .1s ease,opacity .2s ease}
#send:hover{background:var(--accent-hover)}#send:active{transform:scale(.95)}#send:disabled{opacity:.35;cursor:not-allowed;transform:none}
#send:focus-visible{outline:2px solid var(--accent);outline-offset:2px}#send svg{width:18px;height:18px}
.hint{font-size:11px;color:var(--text-muted);text-align:center;margin-top:8px}
@keyframes msgIn{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:translateY(0)}}
@keyframes bounce{from{transform:translateY(0)}to{transform:translateY(-4px);opacity:1}}
@media(prefers-reduced-motion:reduce){.msg-row{animation:none}.typing-dots span{animation:none;opacity:.8}.status-dot{animation:none}}
@media(max-width:640px){#messages{padding:16px}#input-area{padding:12px 16px 16px}.msg-bubble{max-width:85%;font-size:15px}#header{padding:14px 16px}}
</style>
</head>
`

var chatHTMLBody = `<body>
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
`

var chatHTMLScript = `<script>
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
`
