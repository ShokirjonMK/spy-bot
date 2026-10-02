# spy-bot — Ishga tushirish, kamchiliklar tahlili va biznesga yo'naltirish hisoboti

> Fork: `github.com/ShokirjonMK/spy-bot` · Sana: 2026-09-15 · Muhit: Windows 11 + Docker Desktop

---

## 1. Ishga tushirish natijasi (lokal)

| Bosqich | Holat | Izoh |
|---|---|---|
| Klon + konfiguratsiya (`prepare.sh`, `.env`) | ✅ | Mongo/Redis uchun parollar generatsiya qilindi |
| `business-bot` image build | ✅ | Go 1.24.4 + buf, xatosiz kompilyatsiya |
| `creator-bot` image build | ✅ | Xatosiz kompilyatsiya |
| MongoDB konteyner | ✅ healthy | `mongo-init.js` foydalanuvchi yaratdi |
| Redis konteyner | ✅ healthy | |
| `telegram-bot-api` | ✅ healthy | Real `API_ID`/`API_HASH` bilan ishladi |
| `business-bot` runtime | ✅ healthy | 3 migratsiya OK, 5 fayl-worker, gRPC `:50051`, `/metrics` javob beradi |
| `creator-bot` runtime | ⛔ | Faqat @BotFather'dan creator bot tokeni yetishmayapti |

**Xulosa:** 5 servisdan **4 tasi `healthy`** (mongo, redis, telegram-bot-api, business-bot). Kod toza build bo'ladi. To'liq jonli uchtalik test (creator bot → spy bot yaratish → biznes ulanish → o'chirilgan xabarni ko'rish) uchun faqat quyidagilar qoldi:
1. ✅ `TELEGRAM_API_ID` + `TELEGRAM_API_HASH` — berildi
2. ⛔ Creator bot tokeni — @BotFather
3. ⛔ "Business Mode" + "Inline Mode" yoqilgan ikkinchi bot tokeni (bu spy bot bo'ladi)

---

## 2. Arxitektura (qisqacha)

```
Foydalanuvchi ──/start,token──▶ creator-bot ──gRPC AddBot──▶ business-bot (BotManager)
                                    │                              │  har bir spy-bot uchun webhook
                                    ▼                              ▼
                                 MongoDB ◀──────────────── business_message / edited / deleted
                                 Redis (navbat, ratelimit, ignore)
                                    ▲
                        telegram-bot-api (lokal, katta fayllar uchun)
```

- **creator-bot** — foydalanuvchi interfeysi: bot yaratish/o'chirish, til, ro'yxat. `MAX_BOTS_BY_USER=10`.
- **business-bot** — yurituvchi. DB'dagi barcha tokenlar uchun webhook o'rnatadi, xabarlarni saqlaydi, o'chirilgan/o'zgartirilganini egaisiga yuboradi. gRPC server (`:50051`).
- **Bir martalik media** — egasi mediaga reply qilsa, `HasProtectedContent` orqali ajratib olib, Redis navbat (`queue:files`) → worker yuklab yuboradi.
- Kuchli tomonlar: modulli, i18n (en/ru), Prometheus metrikalari, ratelimit, migratsiya tizimi, BSON custom registry.

---

## 3. Kamchiliklar tahlili

### 🔴 Xavfsizlik (kritik)

1. **Bot tokenlari ochiq matnda saqlanadi.** `creator_bot/repository/bot_repository.go:161` — `SecretToken: token` shifrlashsiz Mongo'ga yoziladi, `FindBotByToken` token bo'yicha qidiradi. DB sizib chiqsa — **hujumchi har bir ulangan foydalanuvchining shaxsiy chatlarini egallaydi**. → Tokenlarni KMS/`age`/libsodium bilan shifrlash yoki faqat hash + alohida sirlar do'koni (Vault).
2. **gRPC autentifikatsiyasiz va TLS'siz** (`insecure.NewCredentials()`). Ichki tarmoqda, lekin `AddBot`/`RemoveBot`'ga hech qanday token yo'q — tarmoqqa kirgan har kim bot qo'sha/o'chira oladi.
3. **`AddBot` gRPC timeouti 1 soniya** (`creator_handlers.go:122`). Ichida `GetMe` + `SetWebhook` bor — sekin tarmoqda muntazam timeout → bot yaratildi (DB'da bor) lekin ishlamaydi (rassinxron holat).
4. **Ochiq abusega yo'l qo'yilgan.** Creator botni topgan istalgan odam spy-bot yaratadi. Ro'yxatdan o'tish/limit/to'lov yo'q — spam va suiiste'mol darvozasi.
5. **Yuklangan fayllar diskdan o'chirilmaydi.** `worker_files.go` lokal bot-api yuklagan faylni `os.Open` qiladi-yu, hech qachon `os.Remove` qilmaydi. `telegram-bot-api-data` volumasi (2 GB gacha har fayl) to'lib, disk tugaydi.

### 🟠 Huquqiy va axloqiy (biznes uchun eng muhim)

6. **Mahsulotning o'zi "yashirin kuzatuv" sifatida joylashtirilgan.** Suhbatdoshning roziligisiz uning o'chirgan/o'zgartirgan xabarlarini saqlash ko'p mamlakatlarda (GDPR, O'zbekiston "Shaxsga doir ma'lumotlar to'g'risida"gi qonun, aloqa maxfiyligi) **noqonuniy**. To'lov provayderlari (Stripe/Telegram Payments), App Store/Google Play va Telegram ToS bunday "spyware"ni bloklaydi. Biznes qilish uchun bu birinchi to'siq.
7. **Rozilik (consent) mexanizmi yo'q.** Suhbatdosh hech narsadan xabardor emas. Legitim mahsulot uchun ikki tomon roziligi yoki ochiq-oydin ogohlantirish shart.
8. **Ma'lumotni o'chirish/eksport huquqi yo'q** (GDPR "right to be forgotten"/portability). Foydalanuvchi o'z ma'lumotini o'chira olmaydi.

### 🟡 Ma'lumotlarni saqlash va xarajat

9. **Xabarlarga TTL yo'q.** `telegram_messages_v2` abadiy o'sadi (`mongo_repository.go` — faqat callback-data'da 30 kunlik TTL). Har bir ulangan foydalanuvchining BARCHA yozishmasi cheksiz saqlanadi → xotira portlashi, maxfiylik xavfi, zaxira og'irligi. → Sozlanadigan retention (masalan, 7/30/90 kun) TTL indeksi.
10. **Media Telegram'da qoladi, `file_id` saqlanadi.** Telegram faylni o'chirsa yoki `file_id` eskirsa — tiklab bo'lmaydi. Muhim media uchun tashqi ob'ekt-do'kon (S3/MinIO) kerak.

### 🟢 Texnik / kod sifati

11. **Testlar yo'q** (`_test.go` topilmadi). CI/CD zaif. Biznes uchun regressiya himoyasi shart.
12. ** Off-by-one:** `botsLen > MaxBotsByUser` (`creator_handlers.go:51`) — limitdan 1 ta ortiq bot yaratishga ruxsat beradi (`>=` bo'lishi kerak).
13. **Hardcoded ichki URL'lar** (`business-bot:8080`, `creator-bot:8080/bot`, `50051`) — konfiguratsiyaga chiqarilmagan; K8s/boshqa deploy'da moslashuvchan emas.
14. **Kontekst boshqaruvi** — ko'p joyda `context.Background()`/`context.TODO()` ishlatilib, request bekor qilinishi (cancellation) e'tiborsiz qoldirilgan.
15. **Zaxira/monitoring to'liq emas.** Prometheus bor, lekin Mongo/Redis zaxirasi, alerting (Alertmanager), log agregatsiyasi yo'q.
16. **Bitta business-bot instansi** barcha botlarni ushlaydi (in-memory `bots map`). Gorizontal masshtab (bir nechta replica) uchun bot→instansga taqsimlash (sharding) yo'q.

---

## 4. Biznesga yo'naltirish — yo'l xaritasi

> Asosiy tamoyil: **"yashirin josuslik"dan → "rozilik asosidagi legal SaaS"ga** burilish. Aks holda to'lov, do'kon va qonun to'sadi.

### A bosqich — Huquqiy repozitsiyalash (majburiy, 1-oy)
- Mahsulotni qayta nomlash va joylashtirish. Legitim yo'nalishlar:
  - **Shaxsiy chat arxivi/zaxirasi** — foydalanuvchi O'ZINING xabarlarini saqlaydi (o'chirilganini tiklash o'zi uchun).
  - **Compliance archiving** — kompaniyalar (bank, broker) xodim yozishmalarini qonun talabi bo'yicha arxivlaydi (MiFID II, HIPAA analoglari). Bu yerda **ish beruvchi roziligi** legal asos.
  - **Mijozlarga xizmat / CRM** — biznes akkaunt suhbatlarini jamoa ko'radi, tahlil qiladi.
- Rozilik oqimi: spy-bot ulanganda suhbatga avtomatik ogohlantirish yuborish opsiyasi; ToS + Privacy Policy; GDPR data-delete/export tugmalari.
- Yuridik ko'rik (yurist bilan) — mo'ljal bozor qonunlariga moslik.

### B bosqich — Monetizatsiya (hozir umuman yo'q)
- **Obuna modeli.** Hozir `MAX_BOTS_BY_USER=10` — tekin, flat. Buni bosqichli tariflar bilan almashtirish:

| Tarif | Narx (misol) | Botlar | Retention | Media do'kon | Qo'llab-quvvatlash |
|---|---|---|---|---|---|
| Free | 0 | 1 | 3 kun | ❌ | community |
| Pro | $5/oy | 3 | 30 kun | ✅ 5 GB | email |
| Business | $25/oy | 20 | 90 kun | ✅ 100 GB | priority + SLA |

- **To'lov:** Telegram Stars / Telegram Payments (Uzbek uchun Click/Payme integratsiyasi), yoki Stripe (xalqaro).
- **Billing infratuzilma:** obuna holati, hisob-faktura, grace period, limitlarni real-time tekshirish (creator-bot'da `LenBots` o'rniga "plan limits").

### C bosqich — Mahsulot pishiqligi (2–3 oy)
- **Admin/dashboard** (web): foydalanuvchilar, botlar, obunalar, metrikalar, moderatsiya.
- **Ma'lumot boshqaruvi:** sozlanadigan retention + TTL indeks, GDPR o'chirish/eksport, media uchun S3/MinIO.
- **Xavfsizlik hardening:** token shifrlash (Vault/KMS), gRPC mTLS + auth, sir rotatsiyasi, audit log.
- **Ishonchlilik:** disk fayllarini tozalash (worker'da `os.Remove`), health/alerting, Mongo/Redis zaxira + tiklash tartibi, off-by-one va context tuzatishlari.
- **Sifat:** unit + integration testlar, GitHub Actions CI (build+test+lint+scan), staging muhit.

### D bosqich — Masshtab va o'sish (3–6 oy)
- **Gorizontal masshtab:** business-bot'ni bir nechta replicaga bo'lish, botlarni consistent-hashing bilan taqsimlash; webhook o'rniga kerak bo'lsa long-polling pool.
- **Observability:** Grafana dashboard'lar (allaqachon prod profilda bor), Loki/Tempo.
- **Growth:** referral, ko'p tillilik (uz/ru/en), onboarding sehrgari, self-serve hujjatlar.

### Ustuvorlik (nima birinchi)
1. 🔴 Token shifrlash + gRPC auth + fayl tozalash + xabar TTL (xavfsizlik/xarajat — darhol)
2. 🟠 Huquqiy repozitsiyalash + consent + Privacy Policy (biznesga eshik)
3. 🟢 Obuna/to'lov (daromad)
4. 🔵 Admin panel + testlar + masshtab (o'sish)

---

## 5. Keyingi qadam — jonli sinov

`.env` ga quyidagilarni qo'yib to'liq ishga tushiramiz:
```
TELEGRAM_API_ID=<my.telegram.org dan raqam>
TELEGRAM_API_HASH=<my.telegram.org dan hash>
TELEGRAM_TOKEN=<@BotFather creator bot tokeni>
```
So'ng: `docker compose --profile dev up -d` → creator botga /start → Business+Inline yoqilgan bot tokenini yuborish → o'sha spy-botni Telegram Business → Chatbots'ga ulash → test chatda xabar yozib/o'chirib ko'rish.
