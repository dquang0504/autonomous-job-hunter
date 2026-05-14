---
name: job-hunter
description: Autonomous job hunting agent for junior Golang positions.
metadata:
  openclaw:
    requires:
      bins: ["node", "go"]
      env: ["GROQ_API_KEY", "TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID"]
---

# Job Hunter Skill

Skill này cho phép trợ lý tìm kiếm các vị trí tuyển dụng Golang (Fresher/Junior) và lọc qua các lớp AI để đảm bảo chất lượng.

## Các lệnh hỗ trợ:

### 1. Tìm kiếm job mới (Full Pipeline)
Khi người dùng yêu cầu tìm job, hãy chạy script `job-search.js`. Script này sẽ:
- Cào dữ liệu từ các platform đã cấu hình.
- Lọc theo địa điểm (Ưu tiên HCM/Cần Thơ/Remote).
- Validate bằng AI (Groq) để loại bỏ job Senior/Hà Nội.
- Gửi thông báo về Telegram.

**Cách dùng:**
```bash
node {baseDir}/scripts/scraper-js/job-search.js
```

### 2. Tìm kiếm theo Platform cụ thể
Nếu người dùng chỉ muốn tìm trên một nguồn nhất định (ví dụ: Facebook):
```bash
node {baseDir}/scripts/scraper-js/job-search.js --platform=facebook
```

### 3. Sử dụng Go Scraper (High Speed)
Khi cần cào dữ liệu nhanh hoặc xử lý lượng lớn dữ liệu, hãy sử dụng binary Go:
```bash
{baseDir}/scripts/scraper-go/go-scraper
```

## Hướng dẫn cho Agent:
- Luôn kiểm tra file `.env` để đảm bảo có đủ API Key trước khi chạy.
- Sau khi tìm thấy job, hãy tóm tắt ngắn gọn các job xịn nhất cho người dùng trong cửa sổ chat này ngoài việc gửi Telegram.
- Nếu người dùng hỏi "Có job nào mới không?", hãy tự động chạy lệnh tìm kiếm.
