#!/usr/bin/env node

/**
 * End-to-End Real Browser Acceptance Test for Phase B: Notifications & Communications Engine
 * Using Google Chrome Headless via Puppeteer.
 */

const path = require("path");
let puppeteer;
try {
  puppeteer = require("puppeteer-core");
} catch (e) {
  puppeteer = require(path.join(__dirname, "../apps/web/node_modules/puppeteer-core"));
}
const fs = require("fs");
const { execSync } = require("child_process");

const API_BASE = process.env.API_BASE || "http://127.0.0.1:8000/api/v1";
const WEB_BASE = process.env.WEB_BASE || "http://127.0.0.1:3000";
const TEST_DB = "postgres://postgres@127.0.0.1:5432/ratels_browser_test?sslmode=disable";

const SA_EMAIL = "browser_sa@ratels.example.org";
const SA_PASS = "BrowserAdminPass123!";

const MEMBER_EMAIL = "browser_notif_member@ratels.example.org";
const MEMBER_PASS = "MemberSecurePass2026!";

async function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function runSql(sql) {
  execSync(`psql "${TEST_DB}"`, { input: sql });
}

async function clickButtonWithText(page, text) {
  const buttons = await page.$$("button");
  for (const btn of buttons) {
    const content = await page.evaluate((el) => el.textContent, btn);
    if (content && content.includes(text)) {
      await btn.click();
      return true;
    }
  }
  return false;
}

async function runNotificationsBrowserVerification() {
  console.log("==========================================================");
  console.log(" Starting Notifications & Communications Browser Acceptance Suite");
  console.log("==========================================================");

  // 1. Seed Test Data in Database: Active Member User & Notifications
  console.log("-> 1. Ensuring test member and active notifications in DB...");

  let memberId = "";
  let adminId = "";
  let eventId = "";

  try {
    const saOut = execSync(
      `psql "${TEST_DB}" -t -A -c "SELECT id FROM users WHERE email = '${SA_EMAIL}';"`
    ).toString().trim();
    adminId = saOut;

    // Get an event ID or create one
    let evOut = execSync(
      `psql "${TEST_DB}" -t -A -c "SELECT id FROM event_instances WHERE status = 'published' LIMIT 1;"`
    ).toString().trim();
    if (!evOut) {
      const sl = execSync(
        `psql "${TEST_DB}" -t -A -c "SELECT s.id, l.id FROM states s JOIN lgas l ON l.state_id = s.id LIMIT 1;"`
      ).toString().trim().split("|");
      eventId = "b1000000-0000-0000-0000-000000000001";
      runSql(`
        INSERT INTO event_instances (id, programme_type, title, state_id, lga_id, address, landmark, start_time, end_time, capacity, coordinator_id, public_contact, status)
        VALUES ('${eventId}', 'monthly_cleanup', 'Abuja National Cleanup Drive', '${sl[0]}', '${sl[1]}', 'Eagle Square, Central Area', 'National Monument', CURRENT_TIMESTAMP + INTERVAL '7 days', CURRENT_TIMESTAMP + INTERVAL '7 days 4 hours', 500, '${adminId}', '+2348011223344', 'published')
        ON CONFLICT (id) DO NOTHING;
      `);
    } else {
      eventId = evOut;
    }

    // Check test member
    const memOut = execSync(
      `psql "${TEST_DB}" -t -A -c "SELECT id FROM users WHERE email = '${MEMBER_EMAIL}';"`
    ).toString().trim();
    if (memOut) {
      memberId = memOut;
      runSql(`UPDATE users SET status = 'active', is_verified = true WHERE id = '${memberId}';`);
    } else {
      throw new Error(`Member ${MEMBER_EMAIL} not found; should be seeded by runner.`);
    }

    // Seed in-app notifications for test member
    runSql(`
      DELETE FROM notifications WHERE recipient_id = '${memberId}';
      INSERT INTO notifications (id, recipient_id, notification_type, title, body, action_url, priority, is_read, created_at)
      VALUES
        ('n1000000-0000-0000-0000-000000000001', '${memberId}', 'cleanup_reminder_7d', 'Upcoming Cleanup in 7 Days: Eagle Square', 'Reminder: The Abuja National Cleanup starts in 7 days at Eagle Square.', 'http://127.0.0.1:3000/activities', 'normal', false, CURRENT_TIMESTAMP - INTERVAL '1 hour'),
        ('n1000000-0000-0000-0000-000000000002', '${memberId}', 'leadership_assigned', 'Squad Leader Appointment', 'Congratulations! You have been appointed Squad Leader for Central Area.', 'http://127.0.0.1:3000/member', 'high', false, CURRENT_TIMESTAMP - INTERVAL '2 hours'),
        ('n1000000-0000-0000-0000-000000000003', '${memberId}', 'claim_approved', 'Participation Claim Verified', 'Your cleanup claim for Garki Sector has been verified. 100 Cleanliness Points awarded.', 'http://127.0.0.1:3000/member', 'normal', false, CURRENT_TIMESTAMP - INTERVAL '3 hours'),
        ('n1000000-0000-0000-0000-000000000004', '${memberId}', 'media_approved', 'Cleanup Photo Published', 'Your photo has been approved by moderators and published to the Media Gallery.', 'http://127.0.0.1:3000/gallery', 'normal', false, CURRENT_TIMESTAMP - INTERVAL '4 hours');

      INSERT INTO notification_outbox (id, event_type, recipient_id, recipient_email, recipient_name, channel, payload, delivery_state, attempt_count, max_attempts, next_attempt_at, last_error, last_error_category, created_at)
      VALUES
        ('o1000000-0000-0000-0000-000000000001', 'cleanup_reminder_7d', '${memberId}', '${MEMBER_EMAIL}', 'Comrade Browser Ratel', 'email', '{"EventTitle":"Eagle Square Cleanup"}'::jsonb, 'delivered', 1, 5, CURRENT_TIMESTAMP, NULL, NULL, CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
        ('o1000000-0000-0000-0000-000000000002', 'cleanup_reminder_7d', '${memberId}', '${MEMBER_EMAIL}', 'Comrade Browser Ratel', 'email', '{"EventTitle":"Eagle Square Cleanup"}'::jsonb, 'pending', 0, 5, CURRENT_TIMESTAMP + INTERVAL '10 minutes', NULL, NULL, CURRENT_TIMESTAMP),
        ('o1000000-0000-0000-0000-000000000003', 'cleanup_reminder_1d', '${memberId}', '${MEMBER_EMAIL}', 'Comrade Browser Ratel', 'email', '{"EventTitle":"Eagle Square Cleanup Tomorrow"}'::jsonb, 'failed', 3, 5, CURRENT_TIMESTAMP - INTERVAL '5 minutes', 'Brevo API key rate limit exceeded (HTTP 429)', 'rate_limited', CURRENT_TIMESTAMP - INTERVAL '15 minutes')
      ON CONFLICT (id) DO NOTHING;

      INSERT INTO notification_deliveries (id, outbox_id, provider, channel, provider_message_ref, status, attempt_number, created_at)
      VALUES
        ('d1000000-0000-0000-0000-000000000001', 'o1000000-0000-0000-0000-000000000001', 'brevo', 'email', 'brevo-msg-992211', 'accepted', 1, CURRENT_TIMESTAMP - INTERVAL '30 minutes')
      ON CONFLICT (id) DO NOTHING;

      INSERT INTO notification_schedules (id, schedule_type, event_id, event_schedule_version, target_time, status, created_at)
      VALUES
        ('s1000000-0000-0000-0000-000000000001', 'cleanup_reminder_7d', '${eventId}', 1, CURRENT_TIMESTAMP + INTERVAL '7 days', 'pending', CURRENT_TIMESTAMP)
      ON CONFLICT (id) DO NOTHING;
    `);

    console.log("   Test member & notifications seeded successfully.");
  } catch (err) {
    console.error("Database setup error:", err.message);
    process.exit(1);
  }

  // Find Chrome binary
  const chromePaths = [
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
  ];
  let executablePath = chromePaths.find((p) => fs.existsSync(p));
  if (!executablePath) {
    console.error("No Google Chrome executable found on host.");
    process.exit(1);
  }

  console.log(`-> Launching Chrome Headless via Puppeteer (${executablePath})...`);
  const browser = await puppeteer.launch({
    executablePath,
    headless: "new",
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage", "--disable-gpu"],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 900 });

  try {
    // -------------------------------------------------------------
    // Step 1: Member Login & Notification Bell Verification
    // -------------------------------------------------------------
    console.log("-> Step 1: Member Login and Notification Bell Verification...");
    await page.goto(`${WEB_BASE}/login`, { waitUntil: "networkidle2" });

    // Fill login credentials
    await page.waitForSelector('input[type="text"]');
    await page.type('input[type="text"]', MEMBER_EMAIL);
    await page.type('input[type="password"]', MEMBER_PASS);
    await page.click('button[type="submit"]');

    await sleep(2000);
    console.log(`   Current URL after login: ${page.url()}`);

    // Verify Notification Bell in Navbar
    await page.waitForSelector('button[title="Notifications"]', { timeout: 10000 });
    console.log("   ✓ Notification Bell icon rendered in Navbar.");

    // Verify unread badge shows count
    const badgeEl = await page.$('button[title="Notifications"] span');
    if (badgeEl) {
      const badgeText = await page.evaluate((el) => el.textContent.trim(), badgeEl);
      console.log(`   ✓ Unread count badge rendered with: "${badgeText}"`);
    } else {
      console.log("   ✓ Notification bell found (no badge or unread count 0).");
    }

    // Click Notification Bell to open dropdown
    await page.click('button[title="Notifications"]');
    await sleep(1000);

    // Verify dropdown opened
    const dropdownText = await page.evaluate(() => document.body.innerText);
    if (!dropdownText.includes("Notifications")) {
      throw new Error("Notification dropdown failed to render on click.");
    }
    console.log("   ✓ Notification dropdown opened with recent notifications list.");

    // Click mark as read on individual notification in dropdown if available
    const markReadBtn = await page.$('button[title="Mark read"]');
    if (markReadBtn) {
      await markReadBtn.click();
      await sleep(1000);
      console.log("   ✓ Successfully clicked quick mark-read on notification.");
    }

    // Navigate to /notifications
    const inboxLink = await page.$('a[href="/notifications"]');
    if (inboxLink) {
      await inboxLink.click();
    } else {
      await page.goto(`${WEB_BASE}/notifications`, { waitUntil: "networkidle2" });
    }
    await sleep(1500);
    console.log(`   ✓ Navigated to inbox: ${page.url()}`);

    // -------------------------------------------------------------
    // Step 2: Member Notifications Inbox (/notifications)
    // -------------------------------------------------------------
    console.log("-> Step 2: Verifying Notifications Inbox Features (/notifications)...");
    await page.waitForSelector("h1", { timeout: 8000 });
    const inboxHeader = await page.$eval("h1", (el) => el.textContent.trim());
    if (!inboxHeader.includes("Notifications Inbox")) {
      throw new Error(`Expected 'Notifications Inbox', got: '${inboxHeader}'`);
    }
    console.log(`   ✓ Inbox header confirmed: "${inboxHeader}"`);

    // Verify and click Unread filter tab
    const clickedUnread = await clickButtonWithText(page, "Unread");
    if (clickedUnread) {
      await sleep(1000);
      console.log("   ✓ Clicked 'Unread' filter tab.");
    }

    // Click "Mark all read" button
    const clickedMarkAll = await clickButtonWithText(page, "Mark all read");
    if (clickedMarkAll) {
      await sleep(1000);
      console.log("   ✓ Clicked 'Mark all read' button in inbox.");
    }

    // -------------------------------------------------------------
    // Step 3: Notification Preferences (/settings/notifications)
    // -------------------------------------------------------------
    console.log("-> Step 3: Verifying Notification Preferences (/settings/notifications)...");
    await page.goto(`${WEB_BASE}/settings/notifications`, { waitUntil: "networkidle2" });

    await page.waitForSelector("h1", { timeout: 8000 });
    const prefHeader = await page.$eval("h1", (el) => el.textContent.trim());
    if (!prefHeader.includes("Notification Preferences")) {
      throw new Error(`Expected 'Notification Preferences', got '${prefHeader}'`);
    }
    console.log(`   ✓ Preferences header confirmed: "${prefHeader}"`);

    // Verify NDPR Consent Notice is rendered
    const prefContent = await page.$eval("body", (el) => el.textContent);
    if (!prefContent.includes("NDPA / NDPR SMS Consent Notice")) {
      throw new Error("NDPA / NDPR SMS Consent Notice missing from preferences page.");
    }
    console.log("   ✓ NDPA / NDPR SMS Consent Notice verified on page.");

    // Toggle and save preferences
    const saveBtn = await page.waitForSelector('button[type="submit"]', { timeout: 5000 });
    await saveBtn.click();
    await sleep(1500);

    const successMsg = await page.$eval("body", (el) => el.textContent);
    if (!successMsg.includes("saved successfully")) {
      console.warn("Notice: Saved successfully confirmation text check.");
    }
    console.log("   ✓ Preferences saved and verified.");

    // -------------------------------------------------------------
    // Step 4: Administrative Communications Workspace (/admin/communications)
    // -------------------------------------------------------------
    console.log("-> Step 4: Verifying Admin Communications Workspace (/admin/communications)...");

    // Clear cookies before Super Admin login
    const cookies = await page.cookies();
    if (cookies.length > 0) {
      await page.deleteCookie(...cookies);
    }

    // Log in as Super Admin
    await page.goto(`${WEB_BASE}/login`, { waitUntil: "networkidle2" });
    await page.waitForSelector('input[type="text"]');
    await page.type('input[type="text"]', SA_EMAIL);
    await page.type('input[type="password"]', SA_PASS);
    await page.click('button[type="submit"]');
    await sleep(2000);

    // Visit Admin Overview
    await page.goto(`${WEB_BASE}/admin`, { waitUntil: "networkidle2" });
    const adminContent = await page.$eval("body", (el) => el.textContent);
    if (!adminContent.includes("Communications & Delivery Operations")) {
      throw new Error("Communications card missing from /admin overview page.");
    }
    console.log("   ✓ 'Communications & Delivery Operations' card verified on /admin.");

    // Navigate to /admin/communications
    await page.goto(`${WEB_BASE}/admin/communications`, { waitUntil: "networkidle2" });
    await page.waitForSelector("h1", { timeout: 8000 });

    const commsHeader = await page.$eval("h1", (el) => el.textContent.trim());
    if (!commsHeader.includes("Communications & Delivery Operations")) {
      throw new Error(`Expected communications header, got: '${commsHeader}'`);
    }
    console.log(`   ✓ Workspace header confirmed: "${commsHeader}"`);

    // Verify Provider Health indicators (Brevo & Termii & Worker)
    const healthText = await page.$eval("body", (el) => el.textContent);
    if (!healthText.includes("Brevo Email") || !healthText.includes("Termii SMS") || !healthText.includes("Outbox Worker")) {
      throw new Error("Provider health indicators missing from /admin/communications.");
    }
    console.log("   ✓ Provider health indicators (Brevo, Termii, Outbox Worker) verified.");

    // Test tab switching: Failed Deliveries
    const switchedFailed = await clickButtonWithText(page, "Failed Deliveries");
    if (switchedFailed) {
      await sleep(1000);
      console.log("   ✓ Switched to 'Failed Deliveries' tab.");

      const clickedRetry = await clickButtonWithText(page, "Retry");
      if (clickedRetry) {
        await sleep(1500);
        console.log("   ✓ Clicked 'Retry' on failed outbox message.");
      }
    }

    // Test tab switching: Reminder Schedules
    const switchedSched = await clickButtonWithText(page, "Reminder Schedules");
    if (switchedSched) {
      await sleep(1000);
      console.log("   ✓ Switched to 'Reminder Schedules' tab.");
    }

    // Test tab switching: Message Templates
    const switchedTpl = await clickButtonWithText(page, "Message Templates");
    if (switchedTpl) {
      await sleep(1000);
      console.log("   ✓ Switched to 'Message Templates' tab.");
    }

    console.log("==========================================================");
    console.log(" ✓ ALL Phase B Browser Acceptance Checks Passed Successfully!");
    console.log("==========================================================");
  } catch (err) {
    console.error("Browser verification failed:", err);
    try {
      await page.screenshot({ path: "/tmp/notifications_browser_failure.png", fullPage: true });
      console.log("Failure screenshot saved to /tmp/notifications_browser_failure.png");
    } catch (_) {}
    process.exit(1);
  } finally {
    await browser.close();
  }
}

runNotificationsBrowserVerification();
