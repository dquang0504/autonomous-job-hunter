/**
 * Configuration for OpenClaw Job Search
 */

const path = require('path');

const CONFIG = {
    platforms: {
        active: ['twitter', 'facebook', 'threads', 'indeed', 'topdev', 'itviec', 'vercel', 'cloudflare', 'vietnamworks'],
        inactive: ['linkedin', 'topcv', 'wellfound'],
        cookieBacked: ['twitter', 'facebook', 'threads', 'topdev', 'itviec', 'vercel', 'vietnamworks']
    },
    keywords: [
        'golang'
    ],
    socialSearchKeywords: [
        'golang',
        'go developer',
        'go backend'
    ],
    jobFreshnessDays: 7,
    seenJobsRetentionDays: 120,
    keywordRegex: /\b(golang|go\s?lang|go\s?dev|go\s?engineer|backend\s?go)\b/i,
    antiTitleRegex: /\b(frontend|front-end|ui\/ux|qa|qc|tester|mobile|ios|android|flutter|react native|ba|business analyst|data analyst|data scientist|designer|devops|sysadmin|system admin|security|network|php|wordpress|magento|shopify|sales|marketing|hr)\b/i,
    // Exclude senior-only or 3+ years. Mixed-role posts with junior/fresher roles are handled in filters.
    excludeRegex: /\b(senior|lead|manager|principal|staff|architect|(\d{2,}|[3-9])\s*(\+|plus)?\s*years?)\b/i,
    includeRegex: /\b(fresher|intern|junior|entry[\s-]?level|graduate|trainee)\b/i,

    // Only accept jobs from current year and previous year
    validYears: [new Date().getFullYear(), new Date().getFullYear() - 1],

    locations: {
        primary: ['cần thơ', 'can tho', 'remote', 'từ xa', 'global', 'worldwide', 'anywhere', 'hồ chí minh', 'ho chi minh', 'hcm', 'saigon', 'tphcm'],
        // User requested ONLY Remote or Can Tho or HCM (updated logic)
        secondary: []
    },

    facebookGroups: [
        'https://www.facebook.com/groups/golang.org.vn', // Golang Jobs Viet Nam
        'https://www.facebook.com/groups/1875985159376456', // 'Cần Thơ - IT Jobs'
        'https://www.facebook.com/groups/nodejs.php.python', // 'Tuyển dụng Backend Python, PHP, NodeJS, Golang'
        'https://www.facebook.com/groups/itjobsphp', // 'Tuyển Dụng IT - Việc làm Back-end Java, .NET, Golang, PHP, Python, NodeJS'
        'https://www.facebook.com/groups/ithotjobs.tuyendungit.vieclamcntt.susudev', // IT Hot Jobs
        'https://www.facebook.com/groups/465885632447300', // IT Jobs Group
        'https://www.facebook.com/groups/1556846738967719', // (New Group)
        'https://www.facebook.com/groups/1112083256270739', // (New Group)
        'https://www.facebook.com/groups/1649228812144970/', // Golang (Requested)
        'https://www.facebook.com/groups/391824397049351/', // Golang (Requested)
    ],

    vercelUrl: 'https://vercel.com/dquang0504s-projects/my-portfolio/analytics?period=24h',

    delays: {
        min: 500,
        max: 1500,
        scroll: { min: 200, max: 500 },
        typing: { min: 30, max: 80 }
    },

    paths: {
        cookies: path.resolve(__dirname, '../../.cookies'),
        logs: path.resolve(__dirname, '../../logs'),
        screenshots: path.resolve(__dirname, '../../.tmp/screenshots'),
        seenJobs: path.resolve(__dirname, '../../logs/seen-jobs.json'),
        platformHealth: path.resolve(__dirname, '../../logs/platform-health.json'),
        vercelCache: path.resolve(__dirname, '../../logs/vercel-cache.json'),
        cloudflareCache: path.resolve(__dirname, '../../logs/cloudflare-cache.json')
    },
    cloudflare: {
        accountId: '05bdf9a77d8976b78faf594736063c5d',
        apiToken: process.env.CLOUDFLARE_API_KEY
    }
};

module.exports = CONFIG;
