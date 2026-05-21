/**
 * Screenshot Utility for Debugging
 * Captures screenshots on errors and sends to Telegram
 */

const fs = require('fs');
const path = require('path');

const DEFAULT_CAPTION_LIMIT = 900;

function truncateText(text, maxLength = DEFAULT_CAPTION_LIMIT) {
    const normalized = `${text || ''}`.replace(/\r/g, '').trim();
    if (normalized.length <= maxLength) {
        return normalized;
    }

    return `${normalized.slice(0, maxLength - 3).trimEnd()}...`;
}

function compactErrorMessage(error) {
    const raw = `${error?.message || error || 'Unknown error'}`.replace(/\r/g, '');
    const lines = raw.split('\n').map(line => line.trim()).filter(Boolean);

    if (lines.length === 0) {
        return 'Unknown error';
    }

    const compactLines = [];
    compactLines.push(lines[0]);

    for (const line of lines.slice(1)) {
        if (line === 'Call log:') continue;
        if (line.startsWith('-') || line.includes('intercepts pointer events') || line.includes('Timeout')) {
            compactLines.push(line);
        }
        if (compactLines.length >= 4) break;
    }

    return truncateText(compactLines.join('\n'), 500);
}

class ScreenshotDebugger {
    constructor(reporter) {
        this.reporter = reporter;
        this.screenshotDir = path.join(__dirname, '../../.tmp/screenshots');

        // Ensure directory exists
        if (!fs.existsSync(this.screenshotDir)) {
            fs.mkdirSync(this.screenshotDir, { recursive: true });
        }
    }

    /**
     * Capture screenshot and send to Telegram
     * @param {Page} page - Playwright page object
     * @param {string} context - Context/reason for screenshot (e.g., "topdev-cloudflare")
     * @param {string} message - Optional message to send with screenshot
     */
    async captureAndSend(page, context, message = '') {
        try {
            // Check if page is still open
            if (page.isClosed()) {
                console.log('⚠️ Cannot capture screenshot: page is closed');
                return;
            }

            const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
            const filename = `${context}-${timestamp}.png`;
            const filepath = path.join(this.screenshotDir, filename);

            // Capture screenshot
            try {
                await page.screenshot({
                    path: filepath,
                    fullPage: true,
                    animations: 'disabled',
                    timeout: 15000
                });
            } catch (error) {
                if (page.isClosed()) {
                    throw error;
                }

                if (!/timeout/i.test(error.message || '')) {
                    throw error;
                }

                await page.screenshot({
                    path: filepath,
                    fullPage: false,
                    animations: 'disabled',
                    timeout: 5000
                });
            }

            console.log(`📸 Screenshot saved: ${filename}`);

            // Upload to Supabase Storage
            const db = require('./db');
            let publicUrl = null;
            if (db.isDBEnabled()) {
                publicUrl = await db.uploadScreenshot(filepath, filename);
                if (publicUrl) {
                    console.log(`☁️ Screenshot uploaded to Supabase: ${publicUrl}`);
                }
            }

            // Send to Telegram if reporter available
            if (this.reporter && this.reporter.sendPhoto) {
                const caption = truncateText(message || `🔍 Debug Screenshot: ${context}`);
                await this.reporter.sendPhoto(filepath, caption);
                console.log(`📤 Screenshot sent to Telegram`);
            }

            // Log incident to Supabase DB if it's an error/block context
            if (db.isDBEnabled()) {
                let incidentType = 'error';
                if (context.includes('cloudflare')) incidentType = 'cloudflare_block';
                if (context.includes('timeout')) incidentType = 'timeout';
                if (context.includes('auth')) incidentType = 'login_required';
                
                // Extract platform (usually the prefix before hyphen)
                const platform = context.split('-')[0] || 'unknown';
                
                // Do not log "nojobs" as an incident, it's normal behavior
                if (!context.includes('nojobs')) {
                    await db.logIncident(platform, incidentType, truncateText(message, 500), publicUrl);
                }
            }

            return filepath;
        } catch (error) {
            console.error(`❌ Screenshot capture failed: ${error.message}`);
            return null;
        }
    }

    async capture(page, context, message = '') {
        return this.captureAndSend(page, context, message);
    }

    /**
     * Capture screenshot on Cloudflare detection
     */
    async captureCloudflare(page, platform) {
        const pageTitle = await page.title().catch(() => 'Unknown');
        const url = page.url();

        const message = `🛡️ Cloudflare Challenge Detected\n\n` +
            `Platform: ${platform}\n` +
            `Title: ${pageTitle}\n` +
            `URL: ${url}`;

        return await this.captureAndSend(page, `${platform}-cloudflare`, message);
    }

    /**
     * Capture screenshot on error
     */
    async captureError(page, platform, error) {
        const url = page && !page.isClosed() ? page.url() : 'Unknown';
        const errorSummary = compactErrorMessage(error);

        const message = `❌ Error in ${platform} Scraper\n\n` +
            `Error: ${errorSummary}\n` +
            `URL: ${url}`;

        return await this.captureAndSend(page, `${platform}-error`, message);
    }

    async captureAuthIssue(page, platform, reason = 'Login required or session expired') {
        const url = page && !page.isClosed() ? page.url() : 'Unknown';

        const message = `🔐 Auth issue in ${platform} scraper\n\n` +
            `Reason: ${reason}\n` +
            `URL: ${url}\n\n` +
            `Action: Skipping scraper. Refresh cookies/session if needed.`;

        return await this.captureAndSend(page, `${platform}-auth`, message);
    }

    /**
     * Capture screenshot when no jobs found
     */
    async captureNoJobs(page, platform) {
        const url = page.url();

        const message = `⚠️ No Jobs Found\n\n` +
            `Platform: ${platform}\n` +
            `URL: ${url}`;

        return await this.captureAndSend(page, `${platform}-nojobs`, message);
    }
}

module.exports = ScreenshotDebugger;
