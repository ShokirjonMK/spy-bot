const $ = (id) => document.getElementById(id);
const show = (id) => $(id).classList.remove('hidden');
const hide = (id) => $(id).classList.add('hidden');

async function api(path, opts) {
  const r = await fetch(path, Object.assign({ headers: { 'Content-Type': 'application/json' } }, opts));
  if (r.status === 401) return null;
  if (!r.ok) throw new Error(await r.text());
  const ct = r.headers.get('content-type') || '';
  return ct.includes('json') ? r.json() : r.text();
}

async function init() {
  const pub = await api('/api/public').catch(() => ({}));
  const me = await api('/api/me').catch(() => null);

  if (me && me.user_id) {
    window._me = me;
    await loadDashboard();
    if (me.superadmin) { show('admin'); await loadAdmin(); }
    show('dash');
  } else {
    renderLogin(pub);
    show('login');
  }
}

function renderLogin(pub) {
  if (pub && pub.telegram_ready && pub.login_bot) {
    const s = document.createElement('script');
    s.async = true;
    s.src = 'https://telegram.org/js/telegram-widget.js?22';
    s.setAttribute('data-telegram-login', pub.login_bot);
    s.setAttribute('data-size', 'large');
    s.setAttribute('data-userpic', 'true');
    s.setAttribute('data-request-access', 'write');
    s.setAttribute('data-auth-url', location.origin + '/auth/telegram');
    $('tg-login').appendChild(s);
  } else {
    $('login-note').textContent = 'Telegram Login uchun public domen + @BotFather /setdomain kerak. Lokalda DEV login ishlating.';
  }
  if (pub && pub.dev_mode) show('dev-login');
}

function devLogin() {
  const uid = $('dev-uid').value.trim();
  if (!uid) return;
  location.href = '/auth/dev?uid=' + encodeURIComponent(uid);
}

async function logout() {
  await api('/auth/logout', { method: 'POST' });
  location.reload();
}

async function connectBot() {
  const token = $('bot-token').value.trim();
  const msg = $('bot-msg');
  if (!token) return;
  msg.textContent = '⏳ Tekshirilmoqda...';
  msg.className = 'text-sm text-slate-400';
  try {
    const r = await api('/api/bot/connect', { method: 'POST', body: JSON.stringify({ token }) });
    msg.textContent = '✅ Ulandi: @' + r.username + '. Endi Telegram → Settings → Business → Chatbots\'ga qo\'shing.';
    msg.className = 'text-sm text-green-400';
    $('bot-token').value = '';
    await loadDashboard();
  } catch (err) {
    msg.textContent = '❌ ' + err.message;
    msg.className = 'text-sm text-red-400';
  }
}

async function disconnectBot() {
  if (!confirm('Botni uzishni tasdiqlaysizmi?')) return;
  const msg = $('bot-msg');
  try {
    await api('/api/bot/disconnect', { method: 'POST' });
    msg.textContent = '✅ Uzildi';
    msg.className = 'text-sm text-green-400';
    await loadDashboard();
  } catch (err) {
    msg.textContent = '❌ ' + err.message;
    msg.className = 'text-sm text-red-400';
  }
}

async function loadDashboard() {
  const st = await api('/api/status').catch(() => null);
  if (st) {
    const ok = st.has_bot && st.connections > 0;
    $('status').innerHTML = st.has_bot
      ? `<div class="flex items-center gap-2"><span>${ok ? '🟢' : '🟡'}</span>
         <span>Bot: <b>@${st.bot_username || st.bot_id}</b> · Biznes ulanishlar: <b>${st.connections}</b> · Xabar yuborish: <b>${st.send_messages ? 'ha' : "yo'q"}</b></span></div>`
      : `<div>🔴 Ulangan bot yo'q. Pastda bot tokenini qo'shib ulang.</div>`;
    // ulash bo'limi holati
    if (st.has_bot) {
      $('bot-uname').textContent = '@' + (st.bot_username || st.bot_id);
      show('bot-connected'); hide('bot-connect-form');
    } else {
      hide('bot-connected'); show('bot-connect-form');
    }
  }
  const c = await api('/api/config');
  if (!c) return;
  $('company_name').value = c.company_name || '';
  $('product_info').value = c.product_info || '';
  $('price_list').value = c.price_list || '';
  $('faq').value = c.faq || '';
  $('sales_goal').value = c.sales_goal || 'lead';
  $('language').value = c.language || 'uz';
  $('tone').value = c.tone || 'samimiy';
  $('ai_endpoint').value = c.ai_endpoint || '';
  $('ai_model').value = c.ai_model || '';
  $('ai_format').value = c.ai_format || 'openai';
  $('ai_api_key').value = c.ai_api_key || '';
  $('enabled').checked = !!c.enabled;
  $('reply_auto_new').checked = c.reply_auto_new !== false;
  $('reply_whitelist').value = (c.reply_whitelist || []).join(', ');
}

async function loadAdmin() {
  const d = await api('/api/admin/overview').catch(() => null);
  if (!d) return;
  const s = d.stats || {};
  const card = (label, val, color) => `<div class="bg-slate-800/50 rounded-lg py-3"><div class="text-2xl font-bold ${color}">${val}</div><div class="text-xs text-slate-400">${label}</div></div>`;
  $('admin-stats').innerHTML =
    card('Foydalanuvchi', s.users || 0, 'text-blue-300') +
    card('Botlar', s.bots || 0, 'text-green-300') +
    card('Ulanishlar', s.connections || 0, 'text-emerald-300') +
    card('Faol agent', s.active_agents || 0, 'text-amber-300') +
    card('Faol chatlar', s.ai_chats || 0, 'text-purple-300') +
    card('Xabarlar', s.messages || 0, 'text-slate-300');

  const users = d.users || [];
  $('admin-users').innerHTML =
    `<thead><tr class="text-slate-400 text-left border-b border-slate-700"><th class="py-1">User ID</th><th>Botlar</th><th>Ulanish</th><th>Agent</th><th>Amallar</th></tr></thead><tbody>` +
    (users.length ? users.map(u => `<tr class="border-b border-slate-800 ${u.blocked ? 'opacity-50' : ''}">
      <td class="py-1.5 font-mono">${u.id}${u.blocked ? ' 🚫' : ''}</td><td>${u.bots}</td><td>${u.connections}</td>
      <td>${u.agent_enabled ? '🟢' : '⚪️'}
        <button onclick="toggleAgent(${u.id}, ${!u.agent_enabled})" class="ml-1 text-xs text-blue-400 hover:text-blue-300">${u.agent_enabled ? "o'chir" : 'yoq'}</button>
      </td>
      <td class="whitespace-nowrap">
        <button onclick="viewChats(${u.id})" class="text-xs text-emerald-400 hover:text-emerald-300">suhbatlar</button>
        <button onclick="blockUser(${u.id}, ${!u.blocked})" class="ml-2 text-xs text-amber-400 hover:text-amber-300">${u.blocked ? 'blokdan chiqar' : 'blok'}</button>
        <button onclick="deleteUser(${u.id})" class="ml-2 text-xs text-red-400 hover:text-red-300">o'chir</button>
      </td></tr>`).join('') : `<tr><td colspan="5" class="py-2 text-slate-500">Foydalanuvchi yo'q</td></tr>`) +
    `</tbody>`;

  const bots = d.bots || [];
  $('admin-bots').innerHTML =
    `<thead><tr class="text-slate-400 text-left border-b border-slate-700"><th class="py-1">Bot</th><th>ID</th><th>Egasi</th><th>Ulanish</th><th></th></tr></thead><tbody>` +
    (bots.length ? bots.map(b => `<tr class="border-b border-slate-800">
      <td class="py-1.5">@${b.username}</td><td class="font-mono text-xs">${b.id}</td><td class="font-mono text-xs">${b.owner_id}</td><td>${b.connections}</td>
      <td><button onclick="adminRemoveBot(${b.id})" class="text-xs text-red-400 hover:text-red-300">o'chir</button></td></tr>`).join('') : `<tr><td colspan="5" class="py-2 text-slate-500">Bot yo'q</td></tr>`) +
    `</tbody>`;
}

async function toggleAgent(ownerId, enabled) {
  await api('/api/admin/agent', { method: 'POST', body: JSON.stringify({ owner_id: ownerId, enabled }) }).catch(e => alert(e.message));
  await loadAdmin();
}

async function adminRemoveBot(id) {
  if (!confirm('Bu botni o\'chirishni tasdiqlaysizmi?')) return;
  await api('/api/admin/bot/remove', { method: 'POST', body: JSON.stringify({ id }) }).catch(e => alert(e.message));
  await loadAdmin();
  await loadDashboard();
}

async function blockUser(ownerId, blocked) {
  if (!confirm(blocked ? 'Foydalanuvchini bloklaysizmi? Botlari to\'xtatiladi.' : 'Blokdan chiqaramizmi?')) return;
  await api('/api/admin/user/block', { method: 'POST', body: JSON.stringify({ owner_id: ownerId, blocked }) }).catch(e => alert(e.message));
  await loadAdmin();
}

async function deleteUser(ownerId) {
  if (!confirm('DIQQAT: foydalanuvchi va uning barcha ma\'lumoti (botlar, sozlama, chatlar) o\'chiriladi. Davom etamizmi?')) return;
  await api('/api/admin/user/delete', { method: 'POST', body: JSON.stringify({ owner_id: ownerId }) }).catch(e => alert(e.message));
  await loadAdmin();
  await loadDashboard();
}

function closeChat() { hide('chat-modal'); }

async function viewChats(ownerId) {
  $('chat-modal-title').textContent = 'Foydalanuvchi ' + ownerId + ' — suhbatlar';
  $('chat-modal-body').innerHTML = '<div class="text-slate-400">Yuklanmoqda...</div>';
  show('chat-modal');
  const chats = await api('/api/admin/user/chats?id=' + ownerId).catch(() => []);
  if (!chats || !chats.length) {
    $('chat-modal-body').innerHTML = '<div class="text-slate-500">Suhbat topilmadi</div>';
    return;
  }
  $('chat-modal-body').innerHTML = chats.map(c => `
    <button onclick="viewMessages(${ownerId}, ${c.chat_id}, '${(c.name||'').replace(/'/g,"")}')" class="w-full text-left bg-slate-800/50 hover:bg-slate-700/50 rounded-lg px-3 py-2">
      <div class="flex justify-between"><b>${c.name || c.chat_id}</b><span class="text-xs text-slate-400">${c.count} xabar</span></div>
      <div class="text-xs text-slate-400 truncate">${c.last_text || ''}</div>
    </button>`).join('');
}

async function viewMessages(ownerId, chatId, name) {
  $('chat-modal-title').textContent = (name || chatId) + ' — xabarlar';
  $('chat-modal-body').innerHTML = '<div class="text-slate-400">Yuklanmoqda...</div>';
  const msgs = await api(`/api/admin/user/messages?id=${ownerId}&chat=${chatId}`).catch(() => []);
  const back = `<button onclick="viewChats(${ownerId})" class="text-xs text-blue-400 mb-2">← suhbatlar</button>`;
  if (!msgs || !msgs.length) { $('chat-modal-body').innerHTML = back + '<div class="text-slate-500">Xabar yo\'q</div>'; return; }
  $('chat-modal-body').innerHTML = back + msgs.map(m => {
    const own = m.from_id === ownerId;
    const t = m.text || m.caption || '📎 media';
    return `<div class="flex ${own ? 'justify-end' : 'justify-start'}"><div class="${own ? 'bg-blue-600/30' : 'bg-slate-700/50'} rounded-lg px-3 py-1.5 max-w-[75%]"><div class="text-[10px] text-slate-400">${own ? 'egasi' : 'mijoz'}</div>${t}</div></div>`;
  }).join('');
}

$('cfg-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const wl = $('reply_whitelist').value.split(',').map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n));
  const body = {
    company_name: $('company_name').value,
    product_info: $('product_info').value,
    price_list: $('price_list').value,
    faq: $('faq').value,
    sales_goal: $('sales_goal').value,
    language: $('language').value,
    tone: $('tone').value,
    ai_endpoint: $('ai_endpoint').value,
    ai_model: $('ai_model').value,
    ai_format: $('ai_format').value,
    ai_api_key: $('ai_api_key').value,
    enabled: $('enabled').checked,
    reply_auto_new: $('reply_auto_new').checked,
    reply_whitelist: wl,
  };
  const msg = $('save-msg');
  try {
    await api('/api/config', { method: 'PUT', body: JSON.stringify(body) });
    msg.textContent = '✅ Saqlandi';
    msg.className = 'text-sm text-green-400';
  } catch (err) {
    msg.textContent = '❌ ' + err.message;
    msg.className = 'text-sm text-red-400';
  }
  setTimeout(() => (msg.textContent = ''), 3000);
});

init();
