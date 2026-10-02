# Nazorat — AI Sotuv-Agent platformasi (qurilgan holat)

> spy-bot fork ustiga qurildi · 2026-09-15 · lokal, Docker

## Nima qurildi

Passiv "kuzatuvchi bot"dan → **admin paneldan boshqariladigan AI sotuv-agent SaaS**ga aylantirildi.

### Servislar (hammasi Docker, healthy)
| Servis | Vazifa |
|---|---|
| admin-panel | 🌐 http://localhost:8090 — boshqaruv markazi |
| business-bot | Botlarni yuritadi + AI javob + kuzatuv + ichki API |
| telegram-bot-api | Lokal Bot API (katta fayllar) |
| mongodb / redis | Ma'lumot va navbat |
| mock-ai | Vaqtincha demo AI (haqiqiy model bilan almashtiriladi) |

## Admin panel (hammasi shu yerdan)
1. **Kirish** — Telegram Login (prod) / DEV login (lokal)
2. **🔗 Telegram bot ulash** — bot tokenini qo'yasiz → panel tekshiradi (getMe + Business Mode) → business-bot'da avtomatik ishga tushadi. Uzish ham shu yerda.
3. **🏢 Biznes profili** — kompaniya, mahsulot, narx, FAQ
4. **🎯 Sotuv sozlamasi** — maqsad (lead/buyurtma/qo'ng'iroq), til, uslub
5. **🧠 AI backend** — endpoint, model, format (OpenAI-mos/Ollama), API kalit
6. **💬 Javob doirasi** — yangi/notanishga avto + whitelist

## AI sotuv-agent qanday ishlaydi
```
Mijoz xabar yozadi → business-bot config o'qiydi (Mongo)
  → ruxsatmi? (enabled + yangi/whitelist/.ai on)
  → suhbat tarixi (oxirgi 12 xabar) + biznes-prompt
  → AI endpointga so'rov → javob
  → biznes-ulanish orqali mijozga YUBORADI (egasi nomidan)
```
- Dinamik: chatda `.ai off` / `.ai on` bilan boshqariladi
- Bot o'z javobini kuzatuv logiga tushirmaydi

## Muhim: hamma boshqaruv panelda
Bot ulash/uzish endi faqat admin paneldan (business-bot ichki API `/internal/bot/*` orqali, sir bilan himoyalangan). Qo'lda DB aralashuv shart emas.

## Keyingi (ixtiyoriy)
- Haqiqiy AI modelni panelga qo'yish (hozir mock)
- Prod: Telegram Login domeni (@BotFather /setdomain), token shifrlash, obuna/to'lov
- Ko'p foydalanuvchi: har kim o'z akkaunti bilan kirib, o'z botini ulaydi (arxitektura tayyor)
