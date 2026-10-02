#!/usr/bin/env python3
# Vaqtinchalik MOCK AI — OpenAI-mos /v1/chat/completions.
# Haqiqiy model o'rniga demo javob qaytaradi (zanjirni sinash uchun).
import json
import re
from http.server import BaseHTTPRequestHandler, HTTPServer


def make_reply(messages):
    system = ""
    last_user = ""
    for m in messages:
        if m.get("role") == "system":
            system = m.get("content", "")
        if m.get("role") == "user":
            last_user = m.get("content", "")

    # system prompt'dan kompaniya nomini olishga urinish
    company = "bizning kompaniya"
    mm = re.search(r"Sen (.+?) uchun ishlaydigan", system)
    if mm:
        company = mm.group(1)

    text = (last_user or "").lower()
    if any(w in text for w in ["narx", "qancha", "pul", "цена", "price"]):
        return "Narxlarimiz mahsulotga qarab farq qiladi. Sizni qaysi mahsulot qiziqtiryapti? Aniq narxni darrov aytaman."
    if any(w in text for w in ["salom", "assalom", "hello", "привет", "hi"]):
        return f"Assalomu alaykum! {company} ga xush kelibsiz 😊 Sizga qanday yordam bera olaman?"
    if any(w in text for w in ["ha", "xa", "yes", "roziman", "olaman"]):
        return "Ajoyib! Buyurtmani rasmiylashtirish uchun ism va telefon raqamingizni yozib qoldirsangiz, menejerimiz bog'lanadi."
    return "Rahmat savolingiz uchun! Mahsulotlarimiz haqida batafsil aytaman. Sizga qulay bo'lsa, telefon raqamingizni qoldiring — to'liq taklif yuboraman."


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *a):
        print("[mock-ai]", *a, flush=True)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length) if length else b"{}"
        try:
            body = json.loads(raw)
        except Exception:
            body = {}
        messages = body.get("messages", [])
        reply = make_reply(messages)

        # OpenAI va Ollama ikkalasiga mos javob
        out = {
            "id": "mock-1",
            "object": "chat.completion",
            "model": body.get("model", "mock"),
            "choices": [{"index": 0, "message": {"role": "assistant", "content": reply}, "finish_reason": "stop"}],
            "message": {"role": "assistant", "content": reply},
        }
        data = json.dumps(out).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)
        print("[mock-ai] reply:", reply, flush=True)


if __name__ == "__main__":
    print("[mock-ai] listening on :8000", flush=True)
    HTTPServer(("0.0.0.0", 8000), Handler).serve_forever()
