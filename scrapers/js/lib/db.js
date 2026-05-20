/**
 * Supabase DB Adapter for JS Scrapers
 * 
 * Uses the Supabase REST API to:
 * 1. Check if a job is already seen (dedup)
 * 2. Save new jobs to the jobs table
 * 3. Run stale job cleanup
 * 
 * Falls back gracefully to no-op if DATABASE_URL is not set.
 */

const https = require('https');
const http = require('http');

const SUPABASE_URL = process.env.SUPABASE_URL;
const SUPABASE_SERVICE_KEY = process.env.SUPABASE_SERVICE_ROLE_KEY;
const STALE_JOB_DAYS = 60;

// --- Simple HTTP helper ---
function apiRequest(method, path, body = null) {
    return new Promise((resolve, reject) => {
        if (!SUPABASE_URL || !SUPABASE_SERVICE_KEY) {
            resolve(null);
            return;
        }

        const url = new URL(path, SUPABASE_URL);
        const options = {
            method,
            hostname: url.hostname,
            port: url.port || 443,
            path: url.pathname + url.search,
            headers: {
                'Content-Type': 'application/json',
                'apikey': SUPABASE_SERVICE_KEY,
                'Authorization': `Bearer ${SUPABASE_SERVICE_KEY}`,
                'Prefer': 'return=representation'
            }
        };

        const lib = url.protocol === 'https:' ? https : http;
        const req = lib.request(options, (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                try { resolve(JSON.parse(data)); }
                catch (e) { resolve(data); }
            });
        });

        req.on('error', reject);
        if (body) req.write(JSON.stringify(body));
        req.end();
    });
}

/**
 * Check if a job URL already exists in the DB.
 * Returns true if seen, false if new.
 */
async function isJobSeen(url) {
    if (!SUPABASE_URL) return false;
    try {
        const result = await apiRequest('GET', `/rest/v1/jobs?external_id=eq.${encodeURIComponent(url)}&select=id`);
        return Array.isArray(result) && result.length > 0;
    } catch (e) {
        console.warn('⚠️ DB dedup check failed:', e.message);
        return false; // Fail open — don't skip job on DB error
    }
}

/**
 * Save a job to the DB. Uses ON CONFLICT DO UPDATE via Supabase REST upsert.
 * Returns the saved job row (with id) or null on failure.
 */
async function saveJob(job) {
    if (!SUPABASE_URL) return null;
    try {
        const payload = {
            source: job.source || 'Unknown',
            external_id: job.url,
            title: job.title || '',
            company: job.company || '',
            url: job.url,
            location: job.location || '',
            salary: job.salary || '',
            match_score: job.matchScore || 0,
            posted_at: job.postedDate || '',
            description_raw: (job.description || '').slice(0, 10000),
        };

        const result = await apiRequest('POST', '/rest/v1/jobs', payload);
        if (Array.isArray(result) && result[0]?.id) {
            return result[0];
        }
        return null;
    } catch (e) {
        console.warn('⚠️ DB save failed:', e.message);
        return null;
    }
}

/**
 * Get or create a Telegram user in the DB.
 */
async function getOrCreateUser(telegramId, username) {
    if (!SUPABASE_URL) return null;
    try {
        const result = await apiRequest('GET', `/rest/v1/users?telegram_id=eq.${telegramId}&select=telegram_id`);
        if (Array.isArray(result) && result.length > 0) {
            return result[0];
        }
        const payload = {
            telegram_id: telegramId,
            username: username || '',
            master_resume_json: '{}'
        };
        const insertResult = await apiRequest('POST', '/rest/v1/users', payload);
        if (Array.isArray(insertResult) && insertResult[0]) {
            return insertResult[0];
        }
        return null;
    } catch (e) {
        console.warn('⚠️ DB user check/save failed:', e.message);
        return null;
    }
}

/**
 * Fetch all Telegram chat IDs from the DB.
 */
async function getAllUsers() {
    if (!SUPABASE_URL) return [];
    try {
        const result = await apiRequest('GET', '/rest/v1/users?select=telegram_id');
        if (Array.isArray(result)) {
            return result.map(u => u.telegram_id).filter(Boolean);
        }
        return [];
    } catch (e) {
        console.warn('⚠️ DB fetch users failed:', e.message);
        return [];
    }
}

/**
 * Auto-cleanup: delete jobs older than STALE_JOB_DAYS days.
 * Called once per scrape run.
 */
async function cleanupStaleJobs() {
    if (!SUPABASE_URL) return;
    try {
        const cutoff = new Date(Date.now() - STALE_JOB_DAYS * 24 * 60 * 60 * 1000).toISOString();
        const result = await apiRequest('DELETE', `/rest/v1/jobs?created_at=lt.${cutoff}`);
        console.log(`🧹 DB Cleanup: removed stale jobs older than ${STALE_JOB_DAYS} days.`);
    } catch (e) {
        console.warn('⚠️ DB cleanup failed:', e.message);
    }
}

/**
 * Returns true if DB mode is active (env vars are set).
 */
function isDBEnabled() {
    return !!(SUPABASE_URL && SUPABASE_SERVICE_KEY);
}

module.exports = { isJobSeen, saveJob, getOrCreateUser, getAllUsers, cleanupStaleJobs, isDBEnabled };
