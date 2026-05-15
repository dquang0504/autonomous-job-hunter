/**
 * VietnamWorks Scraper
 * Uses DOM-based scraping for the modern React interface
 */

const CONFIG = require('./config');
const { randomDelay, humanScroll, applyStealthSettings } = require('./lib/stealth');
const { calculateMatchScore, evaluateJob } = require('./lib/filters');
const ScreenshotDebugger = require('./lib/screenshot');

async function scrapeVietnamWorks(page, reporter) {
    console.log('🇻🇳 Searching VietnamWorks...');
    
    const screenshotDebugger = new ScreenshotDebugger(reporter);
    const jobs = [];
    const seenUrls = new Set();
    
    try {
        await applyStealthSettings(page);
        
        // URL provided by user
        const searchUrl = `https://www.vietnamworks.com/viec-lam?q=golang&l=29.15&sortBy=date`;
        console.log(`  🔍 URL: ${searchUrl}`);
        
        await page.goto(searchUrl, { waitUntil: 'networkidle', timeout: 60000 });
        await randomDelay(3000, 5000);
        
        // Initial scroll to load cards
        await humanScroll(page);
        await randomDelay(1000, 2000);
        
        // Extract jobs
        const jobElements = await page.evaluate(() => {
            const results = [];
            // Container for job cards
            const cards = document.querySelectorAll('div.search_list.view_job_item');
            
            cards.forEach(card => {
                try {
                    // 1. Title and Link
                    const titleAnchor = card.querySelector('h2 a');
                    const title = titleAnchor?.innerText?.replace(/^Mới\s+/, '').trim();
                    const url = titleAnchor?.href;
                    
                    if (!title || !url) return;

                    // 2. Company Name
                    const companyAnchor = card.querySelector('div[class*="gUZzDT"] a, a[title][href*="/nha-tuyen-dung/"]');
                    const company = companyAnchor?.innerText?.trim() || 'Unknown Company';

                    // 3. Salary
                    const salarySpan = card.querySelector('span[class*="cfzaBi"]');
                    const salary = salarySpan?.innerText?.trim() || 'Thương lượng';

                    // 4. Location
                    const locationSpan = card.querySelector('span[class*="kVIiDJ"]');
                    const location = locationSpan?.innerText?.trim() || 'Unknown';

                    // 5. Posted Date
                    const dateText = card.querySelector('div[class*="cOFrSM"]')?.innerText?.trim() || 'Hôm nay';

                    // 6. Tech Tags
                    const techTags = Array.from(card.querySelectorAll('label[class*="jJOvRn"]'))
                        .map(lbl => lbl.title || lbl.innerText)
                        .filter(Boolean);

                    results.push({
                        title,
                        url,
                        company,
                        location,
                        salary,
                        dateText,
                        techTags
                    });
                } catch (e) {
                    // Skip malformed
                }
            });
            return results;
        });
        
        console.log(`  📦 Found ${jobElements.length} job cards`);
        
        for (const item of jobElements) {
            if (seenUrls.has(item.url)) continue;
            seenUrls.add(item.url);
            
            console.log(`    🔍 Processing: ${item.title} | ${item.company}`);
            
            // Extract Level from Title
            const lowerTitle = item.title.toLowerCase();
            let level = 'Unknown';
            if (/\b(intern|fresher|trainee|fresher)\b/i.test(lowerTitle)) level = 'Intern/Fresher';
            else if (/\b(junior|entry)\b/i.test(lowerTitle)) level = 'Junior';
            else if (/\b(senior|lead|principal|staff|expert)\b/i.test(lowerTitle)) level = 'Senior+';
            else if (/\b(middle|mid-level|mid)\b/i.test(lowerTitle)) level = 'Middle';

            // Build the job object
            const tagsText = item.techTags.join(', ');
            const job = {
                title: item.title,
                company: item.company,
                url: item.url,
                description: `Salary: ${item.salary} | Level: ${level} | Tags: ${tagsText} | Posted: ${item.dateText}`,
                location: item.location,
                source: 'VietnamWorks',
                techStack: 'Golang',
                postedDate: item.dateText,
                salary: item.salary,
                level: level
            };

            // Use the central evaluation logic
            const evalResult = evaluateJob(job);
            
            // Special Case: If "Golang" is in tech tags, we override 'missing_keyword'
            const hasGolangTag = item.techTags.some(tag => /golang|go\b/i.test(tag));
            let isIncluded = evalResult.include;
            let finalReasons = evalResult.reasons;

            if (!isIncluded && finalReasons.includes('missing_keyword') && hasGolangTag) {
                finalReasons = finalReasons.filter(r => r !== 'missing_keyword');
                if (finalReasons.length === 0) {
                    isIncluded = true;
                }
            }

            if (!isIncluded) {
                console.log(`      ❌ Rejected: ${finalReasons.join(', ')}`);
                continue;
            }
            
            // Match score calculation
            job.matchScore = calculateMatchScore(job);
            jobs.push(job);
            
            console.log(`      ✅ Added! Score: ${job.matchScore} (${level})`);
            if (jobs.length >= 10) break; 
        }
        
    } catch (error) {
        console.error('  ❌ VietnamWorks Scraper Error:', error.message);
        await screenshotDebugger.captureError(page, 'vietnamworks', error);
    }
    
    return {
        jobs,
        status: 'success',
        metrics: {
            scannedCount: seenUrls.size
        }
    };
}

module.exports = { scrapeVietnamWorks };
