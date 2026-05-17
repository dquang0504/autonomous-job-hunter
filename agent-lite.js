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
const JS_SCRAPER_DIR = path.join(__dirname, './skills/job-hunter/scripts/scraper-js');
const GO_SCRAPER_PATH = path.join(__dirname, './skills/job-hunter/scripts/scraper-go/go-scraper');
const TELEGRAM_TOKEN = process.env.TELEGRAM_BOT_TOKEN;
const TELEGRAM_CHAT_ID = process.env.TELEGRAM_CHAT_ID;

const bot = new TelegramBot(TELEGRAM_TOKEN);

const GO_SUPPORTED_PLATFORMS = ['twitter', 'itviec', 'vietnamworks', 'topcv', 'facebook'];

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
                if (jobs && jobs.length > 0) {
                    await sendSimpleReport(jobs, `Go-${p}`);
                }
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
        
    } else if (GO_SUPPORTED_PLATFORMS.includes(platform)) {
        const jobs = await runGoScraper(platform);
        if (jobs && jobs.length > 0) {
            await sendSimpleReport(jobs, `Go-${platform}`);
        }
    } else {
        await runJsScraper(platform);
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
            cwd: path.dirname(GO_SCRAPER_PATH),
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
async function sendSimpleReport(jobs, source) {
    log(`📨 Sending ${jobs.length} jobs from ${source} to Telegram...`);
    for (const job of jobs) {
        const safeDesc = job.description ? job.description.substring(0, 150) + '...' : '';
        const message = `🏢 *${job.company || 'Unknown'}*\n` +
                        `📌 *${job.title}*\n` +
                        `🔗 [View Job](${job.url})\n` +
                        `📍 ${job.location || 'N/A'}\n` +
                        `💰 ${job.salary || 'N/A'}\n` +
                        (job.posted_date ? `📅 ${job.posted_date}\n` : '') +
                        ((source.toLowerCase().includes('facebook') && safeDesc) ? `📄 ${safeDesc}\n` : '') +
                        `🔖 Source: ${source}`;
        
        await bot.sendMessage(TELEGRAM_CHAT_ID, message, { parse_mode: 'Markdown' }).catch(e => log(`⚠️ Telegram error: ${e.message}`));
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
