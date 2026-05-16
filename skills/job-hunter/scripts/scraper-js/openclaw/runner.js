const { evaluateJob } = require('../lib/filters');
const { randomDelay } = require('../lib/stealth');
const { collectTaskResults } = require('./tasks');
const db = require('../lib/db');

async function runOpenClaw({
    context,
    page,
    reporter,
    runPolicy,
    runState,
    telemetry,
    healthTracker = null,
    collectTaskResultsFn = collectTaskResults,
    delayFn = randomDelay
}) {
    const taskResults = await collectTaskResultsFn({ context, page, reporter, runPolicy, runState });
    let allRawJobs = [];

    for (const taskResult of taskResults) {
        telemetry.recordTaskResult(taskResult);

        if (taskResult.staleUrls.length > 0) {
            runState.queueSeenEntries(taskResult.staleUrls, 'stale');
            telemetry.incrementDropReason('stale', taskResult.staleUrls.length);
        }

        if (taskResult.status !== 'failed' && taskResult.status !== 'skipped' && taskResult.rawJobs.length > 0) {
            allRawJobs = allRawJobs.concat(taskResult.rawJobs);
        }
    }

    telemetry.setPipelineCounts({ rawJobs: allRawJobs.length });

    if (healthTracker) {
        const alerts = healthTracker.updateFromTaskResults(taskResults);
        for (const alert of alerts) {
            telemetry.recordHealthAlert(alert);
            if (!runPolicy.isDryRun) {
                await reporter.sendStatus(`⚠️ Platform health alert: ${alert.message}`);
            }
        }
    }

    console.log(`\n📦 Total raw jobs collected: ${allRawJobs.length}`);

    const initialCount = allRawJobs.length;
    const validatedJobs = [];
    
    // BEST PRACTICE: Filter then Save
    // We use deterministic regex-based filtering (evaluateJob)
    for (const job of allRawJobs) {
        const evaluation = evaluateJob(job);
        if (evaluation.include) {
            validatedJobs.push(job);
        } else {
            for (const reason of evaluation.reasons) {
                telemetry.incrementDropReason(reason);
            }
        }
    }

    telemetry.setPipelineCounts({ filteredJobs: validatedJobs.length });
    console.log(`\n🧹 Filtering (Regex): ${initialCount} -> ${validatedJobs.length} jobs (removed senior/irrelevant)`);

    const unseenJobs = [];
    for (const job of validatedJobs) {
        if (runState.seenJobs.has(job.url)) {
            telemetry.incrementDropReason('seen');
            continue;
        }
        unseenJobs.push(job);
    }
    telemetry.setPipelineCounts({ unseenJobs: unseenJobs.length });
    console.log(`\n🔍 Deduplication: ${validatedJobs.length} valid -> ${unseenJobs.length} unseen jobs`);

    if (unseenJobs.length === 0) {
        console.log('ℹ️ No new unseen jobs to process.');
        telemetry.printTaskSummary();
        return {
            taskResults,
            allRawJobs,
            unseenJobs: [],
            validatedNewJobs: [],
            hadNoUnseenJobs: true
        };
    }

    // Process and Persist New Jobs
    const sentUrls = [];
    for (const job of unseenJobs) {
        console.log(`  [Valid] ${job.title?.slice(0, 50)} @ ${job.company}`);

        if (!runPolicy.isDryRun) {
            // 1. Save to Supabase DB (Persistence)
            const dbJob = await db.saveJob(job);
            if (dbJob) {
                console.log(`  💾 Saved to DB (id: ${dbJob.id})`);
            }

            // 2. Send to Telegram (Notification)
            await reporter.sendJobReport(job);
            await delayFn(500, 1000);
        }

        sentUrls.push(job.url);
    }

    runState.queueSeenEntries(sentUrls, 'sent');
    telemetry.setPipelineCounts({ sentJobs: unseenJobs.length });
    await reporter.sendStatus(`✅ Found ${unseenJobs.length} new valid jobs (saved to DB & sent to Telegram).`);
    telemetry.printTaskSummary();

    return {
        taskResults,
        allRawJobs,
        unseenJobs,
        validatedNewJobs: unseenJobs,
        hadNoUnseenJobs: false
    };
}

module.exports = { runOpenClaw };
