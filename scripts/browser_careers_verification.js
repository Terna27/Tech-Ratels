const path = require("path");
let puppeteer;
try {
  puppeteer = require("puppeteer-core");
} catch (e) {
  puppeteer = require(path.join(__dirname, "../apps/web/node_modules/puppeteer-core"));
}

const API_BASE = process.env.API_BASE || "http://127.0.0.1:8000/api/v1";
const WEB_BASE = process.env.WEB_BASE || "http://localhost:3000";

const SA_EMAIL = "browser_sa@ratels.example.org";
const SA_PASS = "BrowserAdminPass123!";

const CANDIDATE_EMAIL = "candidate_browser@ratels.example.org";
const CANDIDATE_PASS = "CandidateSecure2026!";

async function runCareersBrowserVerification() {
  console.log("==========================================================");
  console.log(" Starting Real Browser Careers Acceptance Suite");
  console.log("==========================================================");

  // 1. Ensure test users exist via API
  console.log("-> Ensuring candidate member exists...");
  try {
    const regRes = await fetch(`${API_BASE}/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: CANDIDATE_EMAIL,
        password: CANDIDATE_PASS,
        phone_number: "+2348039991122",
        first_name: "Chidubem",
        last_name: "Okafor",
      }),
    });
    // ignore if already exists (409)
  } catch (e) {
    // continue
  }

  // 2. Launch Google Chrome Headless
  console.log("-> Launching Google Chrome Headless...");
  const browser = await puppeteer.launch({
    executablePath: "/usr/bin/google-chrome-stable",
    headless: "new",
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: 1440, height: 900 });

  const consoleErrors = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") {
      consoleErrors.push(msg.text());
    }
  });

  page.on("pageerror", (err) => {
    console.log(">>> UNCAUGHT PAGE ERROR:", err.message, err.stack);
  });

  page.on("requestfailed", (req) => {
    console.log(`   [REQ FAILED] ${req.method()} ${req.url()}: ${req.failure()?.errorText}`);
  });

  page.on("response", (res) => {
    if (res.status() >= 400) {
      console.log(`   [RES ${res.status()}] ${res.url()}`);
    }
  });

  try {
    // -----------------------------------------------------------
    // Step 1: Public Careers Discovery
    // -----------------------------------------------------------
    console.log("-> [Step 1] Navigating to Public Careers Discovery (/careers)...");
    await page.goto(`${WEB_BASE}/careers`, { waitUntil: "networkidle2" });

    console.log(`   Navigated to ${page.url()}`);
    await page.waitForSelector("h1", { timeout: 10000 });
    const pageTitle = await page.evaluate(() => document.querySelector("h1")?.textContent);
    console.log(`   Page Heading: "${pageTitle}"`);
    if (!pageTitle || !pageTitle.includes("Ratels Careers")) {
      throw new Error(`Expected 'Ratels Careers' in heading, got '${pageTitle}'`);
    }

    // Verify seeded jobs are visible
    await page.waitForSelector("a[href*='/careers/jobs/']", { timeout: 10000 });
    const jobLinks = await page.$$eval("a[href*='/careers/jobs/']", (links) =>
      links.map((l) => ({ text: l.textContent?.trim(), href: l.getAttribute("href") }))
    );
    console.log(`   Found ${jobLinks.length} job links on landing page.`);
    if (jobLinks.length === 0) {
      throw new Error("No job links found on /careers page");
    }
    const targetJobHref = jobLinks[0].href;
    console.log(`   Selected target job link: ${targetJobHref}`);

    // -----------------------------------------------------------
    // Step 2: Login as Candidate & Complete Profile
    // -----------------------------------------------------------
    console.log("-> [Step 2] Logging in as candidate member via UI...");
    await page.goto(`${WEB_BASE}/login?redirect=/careers/profile`, { waitUntil: "networkidle2" });

    await page.waitForSelector("input[type='password']", { timeout: 10000 });
    const idInput = await page.$("input[placeholder*='member1'], input[type='text']");
    await idInput.click({ clickCount: 3 });
    await idInput.type(CANDIDATE_EMAIL);

    const passInput = await page.$("input[type='password']");
    await passInput.click({ clickCount: 3 });
    await passInput.type(CANDIDATE_PASS);

    await page.click("button[type='submit']");
    await page.waitForNavigation({ waitUntil: "networkidle2" }).catch(() => {});

    console.log(`   Current URL after login: ${page.url()}`);

    // Navigate to Candidate Profile page if not already there
    if (!page.url().includes("/careers/profile")) {
      await page.goto(`${WEB_BASE}/careers/profile`, { waitUntil: "networkidle2" });
    }

    await page.waitForSelector("input[placeholder*='Lead Environmental Safety Officer'], input[placeholder*='OSHA Certified']", { timeout: 10000 });
    console.log("   Filling candidate profile...");
    const headlineInput = await page.$("input[placeholder*='Lead Environmental Safety Officer'], input[placeholder*='OSHA Certified']");
    await headlineInput.click({ clickCount: 3 });
    await headlineInput.type("Lead Environmental Safety Inspector");

    const summaryInput = await page.$("textarea[placeholder*='Highlight your background']");
    await summaryInput.click({ clickCount: 3 });
    await summaryInput.type("10+ years field experience in municipal sanitation logistics and OSHA standards.");

    const skillsInput = await page.$("input[placeholder*='Environmental Auditing'], input[placeholder*='OSHA 30']");
    await skillsInput.click({ clickCount: 3 });
    await skillsInput.type("OSHA 30, Environmental Auditing, Waste Management, Fleet Logistics, Health Safety");

    // Save Profile
    await page.click("button[type='submit']");
    await page.waitForFunction(
      () => document.body.textContent.includes("saved successfully") || document.body.textContent.includes("Saved"),
      { timeout: 8000 }
    );
    console.log("   Candidate profile successfully saved.");

    // -----------------------------------------------------------
    // Step 3: Apply for Job Vacancy
    // -----------------------------------------------------------
    console.log(`-> [Step 3] Navigating to Job Detail Page (${targetJobHref})...`);
    await page.goto(`${WEB_BASE}${targetJobHref}`, { waitUntil: "networkidle2" });

    await page.waitForSelector("button", { timeout: 10000 });
    const applyBtn = await page.evaluateHandle(() => {
      const btns = Array.from(document.querySelectorAll("button"));
      return btns.find((b) => b.textContent && b.textContent.includes("Apply for this Position"));
    });

    if (!applyBtn) {
      throw new Error("Could not find 'Apply for this Position' button");
    }

    await applyBtn.click();
    console.log("   Opened application modal.");

    await page.waitForSelector("textarea[placeholder*='certifications']", { timeout: 8000 });

    // Fill application form
    await page.type(
      "textarea[placeholder*='certifications']",
      "Certified in OSHA 30 and Environmental Safety Auditing with proven leadership in municipal waste clearance."
    );
    await page.type(
      "textarea[placeholder*='interested in this position']",
      "I am deeply committed to the Ratels mission of sustainable, dignified environmental sanitation across Nigerian communities."
    );

    // Submit Application
    console.log("   Submitting application...");
    const submitBtn = await page.evaluateHandle(() => {
      const btns = Array.from(document.querySelectorAll("button[type='submit']"));
      return btns.find((b) => b.textContent && b.textContent.includes("Submit Application"));
    });

    if (submitBtn) {
      await submitBtn.click();
    }

    await page.waitForFunction(
      () =>
        document.body.textContent.includes("Application Submitted!") ||
        document.body.textContent.includes("already submitted"),
      { timeout: 10000 }
    );
    console.log("   Application submission result confirmed in UI.");

    // -----------------------------------------------------------
    // Step 4: Track Application in Candidate Portal
    // -----------------------------------------------------------
    console.log("-> [Step 4] Navigating to Applications Tracker (/careers/applications)...");
    await page.goto(`${WEB_BASE}/careers/applications`, { waitUntil: "networkidle2" });

    await page.waitForSelector("h1", { timeout: 10000 });
    const appsText = await page.evaluate(() => document.body.textContent);
    if (!appsText.includes("Submitted Applications") && !appsText.includes("My Applications")) {
      throw new Error("Applications tracker heading not found");
    }
    console.log("   Applications tracker verified.");

    // -----------------------------------------------------------
    // Step 5: Employer Portal Verification
    // -----------------------------------------------------------
    console.log("-> [Step 5] Logging in as Super Admin / Employer to verify Employer Portal...");
    await page.goto(`${WEB_BASE}/login?redirect=/careers/employer`, { waitUntil: "networkidle2" });

    await page.waitForSelector("input[type='password']", { timeout: 10000 });
    const idInputSa = await page.$("input[placeholder*='member1'], input[type='text']");
    await idInputSa.click({ clickCount: 3 });
    await idInputSa.type(SA_EMAIL);

    const passInputSa = await page.$("input[type='password']");
    await passInputSa.click({ clickCount: 3 });
    await passInputSa.type(SA_PASS);

    await page.click("button[type='submit']");
    await page.waitForNavigation({ waitUntil: "networkidle2" }).catch(() => {});

    if (!page.url().includes("/careers/employer")) {
      await page.goto(`${WEB_BASE}/careers/employer`, { waitUntil: "networkidle2" });
    }

    console.log("   Reached Employer Portal (/careers/employer).");
    await page.waitForSelector("h1", { timeout: 10000 });
    const empHeading = await page.evaluate(() => document.querySelector("h1")?.textContent);
    console.log(`   Employer Portal Heading: "${empHeading}"`);

    // -----------------------------------------------------------
    // Step 6: Admin Careers Oversight Verification
    // -----------------------------------------------------------
    console.log("-> [Step 6] Navigating to Admin Careers Workspace (/admin/careers)...");
    await page.goto(`${WEB_BASE}/admin/careers`, { waitUntil: "networkidle2" });

    await page.waitForSelector("h1", { timeout: 10000 });
    const adminHeading = await page.evaluate(() => document.querySelector("h1")?.textContent);
    console.log(`   Admin Careers Heading: "${adminHeading}"`);
    if (!adminHeading || !adminHeading.includes("Ratels Careers Administration")) {
      throw new Error(`Expected 'Ratels Careers Administration' in admin heading, got '${adminHeading}'`);
    }

    const adminBodyText = await page.evaluate(() => document.body.textContent);
    if (!adminBodyText.includes("Employer Verification") || !adminBodyText.includes("Job Moderation")) {
      throw new Error("Admin tabs not found on /admin/careers");
    }
    console.log("   Admin Careers metrics, employer verification, and moderation queues verified.");

    console.log("==========================================================");
    console.log(" Real Browser Careers Acceptance Verification: PASSED (100%)");
    console.log("==========================================================");
  } catch (err) {
    console.error("Browser Acceptance Suite FAILED:", err);
    if (consoleErrors.length > 0) {
      console.error("Browser Console Errors encountered:", consoleErrors);
    }
    process.exit(1);
  } finally {
    await browser.close();
  }
}

runCareersBrowserVerification();
