/**
 * Autonomous Job Hunter - Agent Lite
 * Orchestrates Go and JS scrapers.
 */
require('dotenv').config();
const fs = require('fs');
const path = require('path');
const TelegramBot = require('node-telegram-bot-api');
const { spawn } = require('child_process');

// Config & Paths
const JS_SCRAPER_DIR = path.join(__dirname, './scrapers/js');
const GO_SCRAPER_PATH = path.join(__dirname, './scrapers/go/bin/go-scraper');
const TELEGRAM_TOKEN = process.env.TELEGRAM_BOT_TOKEN;
const TELEGRAM_CHAT_ID = process.env.TELEGRAM_CHAT_ID;

const GO_SUPPORTED_PLATFORMS = ['twitter', 'itviec', 'vietnamworks', 'topcv', 'facebook', 'threads'];

async function log(msg) {
    const timestamp = new Date().toISOString();
    console.log(`[Agent ${timestamp}] ${msg}`);
}

/**
 * Executes a scraper (either JS or Go).
 */
async function executeScraper(platform = 'all') {
    if (platform === 'all') {
        log("🚀 Starting mixed-mode scraping (Go + JS)...");
        
        // 1. Run Go scrapers for supported platforms
        for (const p of GO_SUPPORTED_PLATFORMS) {
            try {
                const jobs = await runGoScraper(p);
                // Go binary handles its own Telegram notifications.
                log(`✅ Go Scraper (${p}) processed ${(jobs || []).length} jobs (self-sent via Go Telegram).`);
            } catch (e) {
                log(`⚠️ Go Scraper (${p}) failed: ${e.message}`);
            }
        }

        // 2. Run JS for the rest
        const allPlatforms = ['twitter', 'facebook', 'threads', 'indeed', 'topdev', 'itviec', 'vercel', 'cloudflare', 'vietnamworks'];
        const jsPlatforms = allPlatforms.filter(p => !GO_SUPPORTED_PLATFORMS.includes(p));
        log(`▶️ Running JS Scrapers for: ${jsPlatforms.join(', ')}`);
        
        try {
            await runJsScraper(jsPlatforms.join(','));
        } catch (e) {
            log(`⚠️ JS Scraper failed: ${e.message}`);
        }
        
    } else {
        const targetPlatforms = platform.split(',').map(p => p.trim()).filter(Boolean);
        const goPlatformsToRun = targetPlatforms.filter(p => GO_SUPPORTED_PLATFORMS.includes(p));
        const jsPlatformsToRun = targetPlatforms.filter(p => !GO_SUPPORTED_PLATFORMS.includes(p));

        for (const p of goPlatformsToRun) {
            try {
                const jobs = await runGoScraper(p);
                // Go binary handles its own Telegram notifications and DB persistence.
                // We only log the count here for visibility.
                log(`✅ Go Scraper (${p}) processed ${(jobs || []).length} jobs (self-sent via Go Telegram).`);
            } catch (e) {
                log(`⚠️ Go Scraper (${p}) failed: ${e.message}`);
            }
        }

        if (jsPlatformsToRun.length > 0) {
            try {
                await runJsScraper(jsPlatformsToRun.join(','));
            } catch (e) {
                log(`⚠️ JS Scraper failed: ${e.message}`);
            }
        }
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

        const child = spawn(path.resolve(GO_SCRAPER_PATH), args, {
            cwd: path.resolve(__dirname, './scrapers/go'),
            env: process.env,
            stdio: ['ignore', 'pipe', 'inherit']
        });

        let output = '';
        child.stdout.on('data', (data) => { output += data; });

        child.on('close', (code) => {
            if (code === 0) {
                try {
                    const jsonMatch = output.match(/\[[\s\S]*\]/);
                    if (!jsonMatch) {
                        log(`ℹ️ No jobs found by Go Scraper (${platform}).`);
                        return resolve([]);
                    }
                    const jobs = JSON.parse(jsonMatch[0]);
                    log(`✅ Go Scraper found ${jobs.length} jobs.`);
                    resolve(jobs);
                } catch (e) {
                    log(`❌ Parse error: ${e.message}`);
                    resolve([]);
                }
            } else {
                log(`⚠️ Go Scraper (${platform}) exited with code ${code}`);
                resolve([]);
            }
        });
    });
}

/**
 * Simple report for Go jobs (since Go scrapers don't send Telegram themselves)
 */
function escapeMarkdown(text) {
    if (!text) return '';
    return text.replace(/[_*[\]()~`>#+\-=|{}.!]/g, '\\$&');
}

async function sendSimpleReport(jobs, source) {
    log(`📨 Sending ${jobs.length} jobs from ${source} to Telegram...`);

    let targetChatIds = [];
    try {
        targetChatIds = await db.getAllUsers();
    } catch (e) {
        log(`⚠️ Failed to load target users from DB: ${e.message}`);
    }

    if (!targetChatIds || targetChatIds.length === 0) {
        if (TELEGRAM_CHAT_ID) {
            targetChatIds = [parseInt(TELEGRAM_CHAT_ID, 10)];
        }
    }

    log(`👥 Broadcasting to ${targetChatIds.length} subscribers: ${targetChatIds.join(', ')}`);

    for (const job of jobs) {
        const safeDesc = job.description ? job.description.substring(0, 150) + '...' : '';
        const lines = [
            `🏢 *${escapeMarkdown(job.company || 'Unknown')}*`,
            job.title ? `📌 *${escapeMarkdown(job.title)}*` : '',
            `🔗 [View Job](${job.url})`,
            job.salary ? `💰 ${escapeMarkdown(job.salary)}` : '',
            `📝 ${escapeMarkdown(job.techstack || 'Golang')}`,
            `📍 ${escapeMarkdown(job.location || 'N/A')}`,
            job.posted_date ? `📅 ${escapeMarkdown(job.posted_date)}` : '',
            (source.toLowerCase().includes('facebook') && safeDesc) ? `📄 ${escapeMarkdown(safeDesc)}` : '',
            `🤖 Match Score: ${escapeMarkdown(job.match_score !== undefined ? job.match_score.toString() : 'N/A')}\\/10`,
            `🔖 Source: ${escapeMarkdown(source)}`,
            `🕒 Found at: ${escapeMarkdown(new Date().toLocaleString('vi-VN', { timeZone: 'Asia/Ho_Chi_Minh' }))}`
        ];

        const message = lines.filter(Boolean).join('\n');
        
        const inlineKeyboard = {
            inline_keyboard: [
                [
                    { text: '🛠️ Refine CV', url: job.url },
                    { text: '🔗 View Job', url: job.url }
                ]
            ]
        };

        for (const chatId of targetChatIds) {
            await bot.sendMessage(chatId, message, { 
                parse_mode: 'MarkdownV2',
                reply_markup: inlineKeyboard
            }).catch(e => log(`⚠️ Telegram error for chat ${chatId}: ${e.message}`));
            await new Promise(r => setTimeout(r, 200));
        }
        
        await new Promise(r => setTimeout(r, 500));
    }
}

async function main() {
    const args = process.argv.slice(2);
    const platform = args.find(a => a.startsWith('--platform='))?.split('=')[1] || 'all';

    try {
        await executeScraper(platform);
        log("🏁 Session finished.");
    } catch (err) {
        log(`❌ Error: ${err.message}`);
        process.exit(1);
    }
}

main();
