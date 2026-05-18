const { loadSeenJobs, saveSeenJobs } = require('../lib/deduplication');
const { isDBEnabled, cleanupStaleJobs } = require('../lib/db');

function createRunState() {
    const seenJobs = loadSeenJobs();
    const pendingSeenEntries = new Map();

    // Run DB cleanup once at start of each run (non-blocking)
    if (isDBEnabled()) {
        cleanupStaleJobs().catch(() => {});
        console.log('🗄️ DB mode enabled: using Supabase for deduplication.');
    } else {
        console.log('📁 DB not configured: using local seen-jobs.json for deduplication.');
    }

    return {
        seenJobs,
        queueSeenEntries(items, status) {
            const now = Date.now();
            for (const item of items || []) {
                const entry = typeof item === 'string'
                    ? { url: item, timestamp: now, status }
                    : { ...item, timestamp: item.timestamp || now, status: item.status || status };

                if (!entry.url) continue;

                const existing = pendingSeenEntries.get(entry.url);
                if (!existing || entry.timestamp >= existing.timestamp) {
                    pendingSeenEntries.set(entry.url, entry);
                }
            }
        },
        persistSeenEntries(isDryRun = false) {
            if (isDryRun || pendingSeenEntries.size === 0) return;
            // Always save to local JSON as a fallback/backup
            saveSeenJobs(Array.from(pendingSeenEntries.values()));
        }
    };
}

module.exports = { createRunState };
