const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const { scrapeVietnamWorks } = require('../execution/scrapers/vietnamworks');

(async () => {
    console.log('🚀 Starting VietnamWorks Scraper Test...');

    const browser = await chromium.launch({
        headless: true,
        args: ['--disable-blink-features=AutomationControlled']
    });

    const context = await browser.newContext({
        userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        viewport: { width: 1280, height: 800 }
    });

    const page = await context.newPage();

    // Mock Reporter
    const reporter = {
        sendStatus: (msg) => console.log(`[STATUS] ${msg}`),
        sendError: (msg) => console.error(`[ERROR] ${msg}`),
        sendTelegramMessage: (msg) => console.log(`[TELEGRAM] ${msg}`)
    };

    try {
        const result = await scrapeVietnamWorks(page, reporter);
        const jobs = result.jobs || [];
        console.log(`\n📦 Total Jobs Found: ${jobs.length}`);
        console.log(JSON.stringify(jobs.slice(0, 5), null, 2)); 
    } catch (error) {
        console.error('❌ Test Failed:', error);
    } finally {
        console.log('✨ Test Complete. Closing browser...');
        await browser.close();
    }
})();
