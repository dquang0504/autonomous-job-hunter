/**
 * Autonomous Job Hunter - Agent Lite
 * Personalized Agentic Orchestration.
 */
require('dotenv').config();
const fs = require('fs');
const path = require('path');
const { Groq } = require('groq-sdk');
const TelegramBot = require('node-telegram-bot-api');
const { spawn } = require('child_process');

// Config & Paths
const SKILL_PATH = path.join(__dirname, '../skills/job-hunter/SKILL.md');
const USER_PROFILE_PATH = path.join(__dirname, '../go-openclaw-automation/base-knowledge.json');
const JS_SCRAPER_DIR = path.join(__dirname, '../skills/job-hunter/scripts/scraper-js');
const GO_SCRAPER_PATH = path.join(__dirname, '../skills/job-hunter/scripts/scraper-go/go-scraper');
const GROQ_API_KEY = process.env.GROQ_API_KEY;
const TELEGRAM_TOKEN = process.env.TELEGRAM_BOT_TOKEN;
const TELEGRAM_CHAT_ID = process.env.TELEGRAM_CHAT_ID;

const groq = new Groq({ apiKey: GROQ_API_KEY });
const bot = new TelegramBot(TELEGRAM_TOKEN);

const GO_SUPPORTED_PLATFORMS = ['twitter', 'itviec', 'vietnamworks', 'topcv'];

async function log(msg) {
    const timestamp = new Date().toISOString();
    console.log(`[Agent ${timestamp}] ${msg}`);
}

/**
 * Executes a scraper (either JS or Go) and returns its results.
 */
async function executeScraper(platform = 'all') {
    if (platform !== 'all' && GO_SUPPORTED_PLATFORMS.includes(platform)) {
        return runGoScraper(platform);
    } else {
        return runJsScraper(platform);
    }
}

async function runJsScraper(platform) {
    return new Promise((resolve, reject) => {
        log(`🚀 Starting JS scraper for: ${platform}`);
        const scraperScript = path.join(JS_SCRAPER_DIR, 'job-search.js');
        const args = [scraperScript];
        if (platform !== 'all') args.push(`--platform=${platform}`);

        const child = spawn('node', args, {
            env: { ...process.env, AGENT_MODE: 'true' },
            stdio: 'inherit'
        });

        child.on('close', (code) => {
            if (code === 0) resolve();
            else reject(new Error(`JS Scraper failed with code ${code}`));
        });
    });
}

async function runGoScraper(platform) {
    return new Promise((resolve, reject) => {
        log(`🚀 Starting high-speed Go scraper for: ${platform}`);
        const args = [`--platform=${platform}`];

        const child = spawn(GO_SCRAPER_PATH, args, {
            env: process.env,
            stdio: ['ignore', 'pipe', 'inherit']
        });

        let output = '';
        child.stdout.on('data', (data) => { output += data; });

        child.on('close', (code) => {
            if (code === 0) {
                try {
                    const jobs = JSON.parse(output);
                    log(`✅ Go Scraper found ${jobs.length} raw jobs.`);
                    const safeTime = new Date().toISOString().replace(/:/g, '-').split('.')[0];
                    const logFile = path.join(__dirname, `../logs/job-search-results-${safeTime}.json`);
                    if (!fs.existsSync(path.dirname(logFile))) fs.mkdirSync(path.dirname(logFile), { recursive: true });
                    fs.writeFileSync(logFile, JSON.stringify({ jobs, source: 'go-' + platform }, null, 2));
                    resolve(jobs);
                } catch (e) {
                    reject(new Error("Failed to parse Go output"));
                }
            } else reject(new Error(`Go Scraper failed`));
        });
    });
}

/**
 * Combined AI Reasoning with User Profile Integration
 */
async function performAgenticReasoning() {
    log("🧠 Starting Personalized Agentic Reasoning...");
    
    if (!GROQ_API_KEY) return;

    // Load User Profile
    let userProfile = {};
    if (fs.existsSync(USER_PROFILE_PATH)) {
        userProfile = JSON.parse(fs.readFileSync(USER_PROFILE_PATH, 'utf-8')).personal_profile;
        log("👤 User profile loaded for personalized CV tips.");
    }

    const resultsDir = path.join(__dirname, '../logs');
    if (!fs.existsSync(resultsDir)) return;

    const resultFiles = fs.readdirSync(resultsDir)
        .filter(f => f.startsWith('job-search-results-') && f.endsWith('.json'))
        .sort().reverse();

    if (resultFiles.length === 0) return;

    const rawData = JSON.parse(fs.readFileSync(path.join(resultsDir, resultFiles[0]), 'utf-8'));
    const jobs = rawData.jobs || [];

    if (jobs.length === 0) return;

    log(`🔍 Analyzing ${jobs.length} high-potential jobs...`);

    const prompt = `
You are the "Autonomous Job Hunter" Agent. 
Your candidate is ${userProfile.full_name || 'the user'}. 

### Candidate Skills & Experience:
${JSON.stringify(userProfile, null, 2)}

### Your Task:
1. FAIR ANALYSIS: For each job, identify objective "Green Flags" (pros) and "Red Flags" (cons/risks). 
2. PERSONALIZED CV TIPS: Based on the candidate's actual skills above, suggest exactly what they should highlight or modify in their CV to match THIS specific job. Don't give generic advice.
3. TARGET: Ensure the jobs are for Intern, Fresher, or Junior levels.
4. LOCATIONS: Prioritize Ho Chi Minh City, Can Tho, or Remote positions.

### Jobs to analyze:
${JSON.stringify(jobs.slice(0, 15).map(j => ({ title: j.title, company: j.company, location: j.location, description: (j.description || "").slice(0, 1000) })), null, 2)}

### Output Format (JSON only):
{
  "matches": [
    {
      "title": "...",
      "company": "...",
      "location": "...",
      "fair_analysis": {
         "green_flags": ["...", "..."],
         "red_flags": ["...", "..."]
      },
      "personalized_cv_tips": "Specific advice based on comparing their skills with this JD.",
      "url": "..." 
    }
  ]
}
`;

    try {
        const completion = await groq.chat.completions.create({
            messages: [{ role: "user", content: prompt }],
            model: "llama-3.3-70b-versatile",
            response_format: { type: "json_object" }
        });

        const result = JSON.parse(completion.choices[0].message.content);
        const matches = result.matches || result.best_matches || [];

        if (matches.length > 0) {
            log(`✨ AI processed ${matches.length} matches.`);
            await sendFormattedReports(matches);
        }
    } catch (err) {
        log(`❌ Reasoning error: ${err.message}`);
    }
}

async function sendFormattedReports(matches) {
    // Message 1: Summary List
    let summary = `🛡️ *PERSONALIZED SELECTION FOR YOU*\n\n`;
    matches.forEach((m, i) => {
        summary += `${i+1}. *${m.title}* @ ${m.company} (${m.location})\n`;
    });
    summary += `\n_Đã đối chiếu với kỹ năng của bạn. Chi tiết phân tích bên dưới..._`;
    
    await bot.sendMessage(TELEGRAM_CHAT_ID, summary, { parse_mode: 'Markdown' });

    // Message 2: Detailed Analysis
    for (const m of matches) {
        let detail = `🧐 *Phân tích: ${m.title}*\n🏢 ${m.company}\n\n`;
        
        detail += `✅ *Green Flags:*\n${m.fair_analysis.green_flags.map(f => "- " + f).join('\n')}\n\n`;
        detail += `⚠️ *Lưu ý (Red Flags):*\n${m.fair_analysis.red_flags.map(f => "- " + f).join('\n')}\n\n`;
        detail += `📄 *Mẹo chỉnh CV cho bạn:*\n_${m.personalized_cv_tips}_\n\n`;
        
        await bot.sendMessage(TELEGRAM_CHAT_ID, detail, { parse_mode: 'Markdown' });
        await new Promise(r => setTimeout(r, 500));
    }
    
    log("📨 Personalized reports sent to Telegram.");
}

async function main() {
    const args = process.argv.slice(2);
    const platform = args.find(a => a.startsWith('--platform='))?.split('=')[1] || 'all';

    try {
        await executeScraper(platform);
        if (!process.argv.includes('--dry-run')) {
            await performAgenticReasoning();
        }
        log("🏁 Session finished.");
    } catch (err) {
        log(`❌ Error: ${err.message}`);
        process.exit(1);
    }
}

main();
