#  Autonomous Job Hunter

**Autonomous AI job hunting agent for junior-oriented Golang positions.**

This project is an **Agentic Scraper** designed to hunt for Golang jobs (Intern/Fresher/Junior) across multiple platforms. It is optimized for running on **GitHub Actions** (free tier) and uses **Groq AI (Llama 3)** as its reasoning engine.

---

## 🚀 Key Features

- **Agentic Reasoning**: Uses Groq AI to perform human-like analysis on job descriptions. It doesn't just filter by keywords; it understands the context and "thinks" like a recruiter.
- **Multi-platform Scraping**: Automates Facebook Groups, X/Twitter, Threads, Indeed, TopDev, ITViec, VietnamWorks, and more.
- **Autonomous Filtering**: Strictly enforces your preferences:
  - **Level**: Prioritizes Junior/Fresher; smartly rejects Senior/Lead roles.
  - **Location**: Focuses on Ho Chi Minh City, Can Tho, or Remote.
  - **Freshness**: Only looks for jobs posted in the last 7 days.
- **OpenClaw-Ready Architecture**: Built using a modular **Skill** structure. If you ever upgrade to a full [OpenClaw](https://github.com/openclaw/openclaw) gateway, your scrapers are already organized and ready to be dropped in.
- **Lightweight & Fast**: Optimized for GitHub Actions. No heavy daemons or complex setups required.

---

## 🛠️ Architecture

The project is orchestrated by the **Agent Lite** engine:

1.  **Planning**: `execution/agent-lite.js` reads the goal from `skills/job-hunter/SKILL.md`.
2.  **Execution (Skills)**: It triggers the high-performance scrapers located in `skills/job-hunter/scripts/`.
    - **JS Scrapers**: Playwright-based workers for complex web portals.
    - **Go Scrapers**: Native high-speed workers for API-driven platforms.
3.  **Reasoning (The Brain)**: Raw data is sent to Groq AI. The Agent performs a multi-step reasoning loop to find the best "match" for you.
4.  **Reporting**: A curated "Agent's Verdict" is sent to your Telegram, summarizing why the selected jobs are worth your time.

---

## 📂 Project Structure

```text
/
├── skills/
│   └── job-hunter/           # The core "Skill" package
│       ├── SKILL.md          # Agent instructions & metadata
│       └── scripts/          # Scraper implementations
│           ├── scraper-js/   # Node.js + Playwright scrapers
│           └── scraper-go/   # High-speed Go scrapers
├── execution/
│   └── agent-lite.js         # The main Agentic orchestrator
├── logs/                     # Persistence & Deduplication data
└── .github/workflows/        # Automation (Cron: Every 4 hours)
```

---

## ⚙️ Setup & Usage

### Requirements
- Node.js 20+
- Playwright Chromium
- Groq API Key (Free tier works perfectly)
- Telegram Bot Token & Chat ID

### Local Run
```bash
npm install
npm run search
```

### GitHub Actions
The bot runs automatically every 4 hours. You can track its "Health" and "Seen Jobs" in the GitHub Actions artifacts and cache.

---

##  Inspired by OpenClaw
This project follows the **Lobster Way** of organizing AI skills. It is a lightweight version designed for serverless environments while remaining 100% compatible with the official OpenClaw skill specification.

---

## 📝 Status
- **Agentic Loop**: ACTIVE (via Groq)
- **Scrapers**: ACTIVE (Facebook, Twitter, Threads, ITViec, etc.)
- **Go Integration**: READY (Modular binary support)
- **Framework**: Lightweight Agent (No persistent gateway required)
