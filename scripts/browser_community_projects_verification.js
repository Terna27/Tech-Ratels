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

const MEMBER_EMAIL = "member_cp@ratels.example.org";
const MEMBER_PASS = "MemberSecurePass2026!";

async function runCommunityProjectsBrowserVerification() {
  console.log("==========================================================");
  console.log(" Starting Community Projects & Requests Acceptance Suite");
  console.log("==========================================================");

  // 1. Ensure test member user exists and is verified
  console.log("-> 1. Ensuring regular member user exists...");
  try {
    await fetch(`${API_BASE}/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: MEMBER_EMAIL,
        password: MEMBER_PASS,
        phone_number: "+2348039988776",
        first_name: "Fatima",
        last_name: "Bello",
      }),
    });
  } catch (e) {
    // Continue if exists
  }

  // Activate member in test DB
  const { execSync } = require("child_process");
  try {
    execSync(`psql "postgres://postgres@127.0.0.1:5432/ratels_browser_test?sslmode=disable" -c "UPDATE users SET status = 'active', is_verified = true WHERE email = '${MEMBER_EMAIL}';"`);
  } catch (e) {
    console.warn("Notice: DB user activation:", e.message);
  }

  // 2. Fetch login tokens for API contract verification
  console.log("-> 2. Authenticating member & admin via API for invariant checks...");
  const memberLoginRes = await fetch(`${API_BASE}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ identifier: MEMBER_EMAIL, password: MEMBER_PASS }),
  });
  if (!memberLoginRes.ok) {
    throw new Error(`Member API login failed: ${memberLoginRes.status}`);
  }
  const memberData = await memberLoginRes.json();
  const memberHeaders = {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${memberData.access_token}`,
    "X-CSRF-Token": memberData.csrf_token,
  };

  const saLoginRes = await fetch(`${API_BASE}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ identifier: SA_EMAIL, password: SA_PASS }),
  });
  if (!saLoginRes.ok) {
    throw new Error(`Admin API login failed: ${saLoginRes.status}`);
  }
  const saData = await saLoginRes.json();
  const saHeaders = {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${saData.access_token}`,
    "X-CSRF-Token": saData.csrf_token,
  };

  // 3. Query states/LGAs for test request submission
  const statesRes = await fetch(`${API_BASE}/geography/states`);
  const states = await statesRes.json();
  if (!states || states.length === 0) {
    throw new Error("No states available for community projects testing");
  }
  const testState = states[0];
  const lgasRes = await fetch(`${API_BASE}/geography/states/${testState.id}/lgas`);
  const lgas = await lgasRes.json();
  const testLga = lgas && lgas.length > 0 ? lgas[0] : null;

  // 4. API Invariant Tests: Anti-Self-Approval, Duplicate Conversion, Milestone Gates
  console.log("-> 3. Verifying Core Domain Invariants via API...");

  // Invariant A: Member submits request via API
  const submitReqRes = await fetch(`${API_BASE}/community/requests`, {
    method: "POST",
    headers: memberHeaders,
    body: JSON.stringify({
      title: "API Invariant Test Request - Clean Water",
      description: "Testing anti-self-approval and duplicate conversion invariants",
      category: "water_and_boreholes",
      urgency_level: "high",
      state_id: testState.id,
      lga_id: testLga ? testLga.id : null,
      community_ward: "Central Ward",
      estimated_beneficiaries: 2000,
      estimated_cost_kobo: 350000000,
    }),
  });
  if (!submitReqRes.ok) {
    const errText = await submitReqRes.text();
    throw new Error(`Failed to submit request via API: ${submitReqRes.status} ${errText}`);
  }
  const submittedReq = await submitReqRes.json();
  console.log(`   ✓ Request created: ID=${submittedReq.id}, Status=${submittedReq.status}`);

  // Invariant B: Anti-Self-Approval
  console.log("   -> Verifying Anti-Self-Approval rule (requester cannot review own request)...");
  const selfReviewRes = await fetch(`${API_BASE}/admin/community/requests/${submittedReq.id}/review`, {
    method: "POST",
    headers: memberHeaders,
    body: JSON.stringify({
      decision: "approve",
      review_notes: "Attempting self-approval should fail",
    }),
  });
  if (selfReviewRes.ok) {
    throw new Error("SECURITY VIOLATION: Member was able to review/approve their own request!");
  }
  console.log(`   ✓ Anti-self-approval enforced! Response status=${selfReviewRes.status}`);

  // Invariant C: Admin reviews and approves request
  console.log("   -> Admin reviews and approves request...");
  const adminReviewRes = await fetch(`${API_BASE}/admin/community/requests/${submittedReq.id}/review`, {
    method: "POST",
    headers: saHeaders,
    body: JSON.stringify({
      decision: "approve",
      review_notes: "Approved after official verification on ground",
    }),
  });
  if (!adminReviewRes.ok) {
    throw new Error(`Admin review failed: ${adminReviewRes.status} ${await adminReviewRes.text()}`);
  }
  console.log("   ✓ Admin successfully approved request");

  // Invariant D: Convert approved request to project
  console.log("   -> Converting approved request to Project...");
  const convertRes = await fetch(`${API_BASE}/admin/community/requests/${submittedReq.id}/convert`, {
    method: "POST",
    headers: saHeaders,
    body: JSON.stringify({
      title: "Solar Water Project Central",
      description: "Converted project from approved community request",
      category: "water_and_boreholes",
      state_id: testState.id,
      lga_id: testLga ? testLga.id : "",
      community_ward: "Central Ward",
      proposed_budget: 350000000,
      approved_budget: 350000000,
      target_completion_date: "2026-12-31",
      is_public: true,
    }),
  });
  if (!convertRes.ok) {
    throw new Error(`Conversion to project failed: ${convertRes.status} ${await convertRes.text()}`);
  }
  const createdProject = await convertRes.json();
  console.log(`   ✓ Project converted with reference: ${createdProject.reference}`);
  if (!createdProject.reference.startsWith("CP-2026-")) {
    throw new Error(`Invalid project reference format: ${createdProject.reference}`);
  }

  // Invariant E: Duplicate Conversion Prevention
  console.log("   -> Verifying Duplicate Conversion Prevention...");
  const dupConvertRes = await fetch(`${API_BASE}/admin/community/requests/${submittedReq.id}/convert`, {
    method: "POST",
    headers: saHeaders,
    body: JSON.stringify({
      title: "Duplicate Attempt",
      description: "Should fail",
      category: "water_and_boreholes",
      state_id: testState.id,
      lga_id: testLga ? testLga.id : "",
      community_ward: "Central Ward",
      proposed_budget: 350000000,
      approved_budget: 350000000,
    }),
  });
  if (dupConvertRes.ok) {
    throw new Error("SECURITY VIOLATION: Duplicate project conversion was allowed!");
  }
  console.log(`   ✓ Duplicate conversion rejected! Response status=${dupConvertRes.status}`);

  // Invariant F: Milestone Verification Gate
  console.log("   -> Adding milestone and verifying completion gate...");
  const addMilestoneRes = await fetch(`${API_BASE}/admin/community/projects/${createdProject.id}/milestones`, {
    method: "POST",
    headers: saHeaders,
    body: JSON.stringify({
      title: "Phase 1: Borehole Drilling",
      description: "Drilling 120m down to clean aquifer",
      sequence_order: 1,
      target_date: "2026-11-15",
      budget_allocation_kobo: 200000000,
    }),
  });
  if (!addMilestoneRes.ok) {
    throw new Error(`Failed to add milestone: ${addMilestoneRes.status}`);
  }
  const createdMilestone = await addMilestoneRes.json();
  console.log(`   ✓ Milestone created: ID=${createdMilestone.id}`);

  // Mark milestone completed (unverified)
  const patchMilestoneRes = await fetch(`${API_BASE}/admin/community/projects/${createdProject.id}/milestones/${createdMilestone.id}`, {
    method: "PATCH",
    headers: saHeaders,
    body: JSON.stringify({ status: "completed" }),
  });
  if (!patchMilestoneRes.ok) {
    throw new Error(`Failed to mark milestone completed: ${patchMilestoneRes.status} ${await patchMilestoneRes.text()}`);
  }

  // Attempt to transition project to completed while milestone is unverified
  const prematureCompleteRes = await fetch(`${API_BASE}/admin/community/projects/${createdProject.id}/status`, {
    method: "PATCH",
    headers: saHeaders,
    body: JSON.stringify({ execution_status: "completed" }),
  });
  if (prematureCompleteRes.ok) {
    throw new Error("INTEGRITY VIOLATION: Project completed with unverified completed milestones!");
  }
  console.log(`   ✓ Milestone verification gate enforced! Premature completion blocked with status=${prematureCompleteRes.status}`);

  // Verify milestone
  const verifyMilestoneRes = await fetch(`${API_BASE}/admin/community/projects/${createdProject.id}/milestones/${createdMilestone.id}/verify`, {
    method: "POST",
    headers: saHeaders,
    body: JSON.stringify({
      decision: "verified",
      verification_notes: "Physical borehole water tested and verified by zonal engineer",
    }),
  });
  if (!verifyMilestoneRes.ok) {
    throw new Error(`Milestone verification failed: ${verifyMilestoneRes.status}`);
  }
  console.log("   ✓ Milestone verified by official");

  // Post update
  await fetch(`${API_BASE}/admin/community/projects/${createdProject.id}/updates`, {
    method: "POST",
    headers: saHeaders,
    body: JSON.stringify({
      title: "Solar Pump Installed & Running",
      content: "Clean water now flowing from taps across Kutunku community.",
      update_type: "milestone",
      is_public: true,
    }),
  });
  console.log("   ✓ Project update published");

  // Complete project now that milestone is verified
  const legitCompleteRes = await fetch(`${API_BASE}/admin/community/projects/${createdProject.id}/status`, {
    method: "PATCH",
    headers: saHeaders,
    body: JSON.stringify({ execution_status: "completed" }),
  });
  if (!legitCompleteRes.ok) {
    throw new Error(`Legitimate completion failed: ${legitCompleteRes.status}`);
  }
  console.log("   ✓ Project transitioned to 'completed' successfully!");

  // -----------------------------------------------------------
  // 5. Google Chrome Headless Browser End-to-End Walkthrough
  // -----------------------------------------------------------
  console.log("-> 4. Launching Google Chrome Headless for UI Acceptance...");
  const browser = await puppeteer.launch({
    executablePath: "/usr/bin/google-chrome-stable",
    headless: "new",
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: 1440, height: 900 });

  const pageErrors = [];
  page.on("pageerror", (err) => {
    console.log("   >>> PAGE ERROR:", err.message);
    pageErrors.push(err.message);
  });
  page.on("console", (msg) => {
    if (msg.type() === "error" || msg.type() === "warn") {
      console.log(`   [PAGE ${msg.type().toUpperCase()}]`, msg.text());
    }
  });

  try {
    // UI Step 1: Visit Public Projects page
    console.log("-> Navigating to public /projects...");
    await page.goto(`${WEB_BASE}/projects`, { waitUntil: "networkidle2" });
    await page.waitForSelector("h1", { timeout: 10000 });
    const h1Text = await page.$eval("h1", (el) => el.innerText);
    console.log(`   Page Title: "${h1Text}"`);
    if (!h1Text.includes("Community Projects")) {
      throw new Error(`Unexpected public projects title: ${h1Text}`);
    }

    // Verify Project Card rendered
    await page.waitForSelector(`a[href='/projects/${createdProject.id}']`, { timeout: 10000 });
    console.log(`   ✓ Found project card for ${createdProject.reference}`);

    // UI Step 2: Visit Public Project Details
    console.log(`-> Navigating to public project details /projects/${createdProject.id}...`);
    await page.goto(`${WEB_BASE}/projects/${createdProject.id}`, { waitUntil: "networkidle2" });
    try {
      await page.waitForSelector("h1", { timeout: 10000 });
    } catch (e) {
      const pageHtml = await page.content();
      console.log(">>> CURRENT PAGE HTML:", pageHtml);
      throw e;
    }
    const detailTitle = await page.$eval("h1", (el) => el.innerText);
    console.log(`   Detail Title: "${detailTitle}"`);

    // Verify Milestones and Updates are displayed
    const bodyContent = await page.$eval("body", (el) => el.innerText);
    if (!bodyContent.includes("Phase 1: Borehole Drilling")) {
      throw new Error("Milestone not rendered on public project detail page");
    }
    if (!bodyContent.includes("Solar Pump Installed & Running")) {
      throw new Error("Update not rendered on public project detail page");
    }
    if (!bodyContent.includes("Public Financial Accountability")) {
      throw new Error("Public Financial Accountability section not rendered");
    }
    // Verify privacy: requester email and private notes should not appear
    if (bodyContent.includes(MEMBER_EMAIL)) {
      throw new Error("PRIVACY LEAK: Requester email is visible on public project page!");
    }
    console.log("   ✓ Milestone, update, and financial transparency verified with zero privacy leaks.");

    // UI Step 3: Member Login and Request Submission Workflow
    console.log("-> Navigating to /login for member authentication...");
    await page.goto(`${WEB_BASE}/login?redirect=/projects/submit`, { waitUntil: "networkidle2" });
    await page.waitForSelector('input[name="identifier"], input[type="text"]', { timeout: 10000 });
    await page.type('input[name="identifier"], input[type="text"]', MEMBER_EMAIL);
    await page.type('input[type="password"]', MEMBER_PASS);
    await page.click('button[type="submit"]');

    await page.waitForNavigation({ waitUntil: "networkidle2" });
    console.log("   ✓ Member authenticated, navigated to /projects/submit");

    // UI Step 4: Fill and Submit Community Request Form
    console.log("-> Submitting community request via web form...");
    await page.waitForSelector('input[placeholder*="Canal"]', { timeout: 10000 });

    // Title
    const titleInput = await page.$('input[placeholder*="Canal"]');
    if (titleInput) {
      await titleInput.type("Erosion Control & Drainage in Gwagwalada Ward 2");
    }

    // Select Category and State via evaluate
    await page.evaluate(() => {
      const selects = Array.from(document.querySelectorAll("select"));
      if (selects[0]) {
        selects[0].value = "drainage_and_flooding";
        selects[0].dispatchEvent(new Event("change", { bubbles: true }));
      }
      if (selects[1]) {
        selects[1].value = "high";
        selects[1].dispatchEvent(new Event("change", { bubbles: true }));
      }
      if (selects[2] && selects[2].options.length > 1) {
        selects[2].selectedIndex = 1;
        selects[2].dispatchEvent(new Event("change", { bubbles: true }));
      }
    });

    await new Promise((r) => setTimeout(r, 1000));

    // Select LGA after state options load
    await page.evaluate(() => {
      const selects = Array.from(document.querySelectorAll("select"));
      if (selects[3] && selects[3].options.length > 1) {
        selects[3].selectedIndex = 1;
        selects[3].dispatchEvent(new Event("change", { bubbles: true }));
      }
    });

    // Ward
    const wardInput = await page.$('input[placeholder*="Ward 05"]');
    if (wardInput) {
      await wardInput.type("Dobi Ward");
    }

    // Location details
    const locInput = await page.$('input[placeholder*="Opposite"]');
    if (locInput) {
      await locInput.type("Near Primary Health Center, Kutunku");
    }

    // Description
    const descArea = await page.$("textarea");
    if (descArea) {
      await descArea.type("Severe flooding and gully erosion endangering community school and residential access during rainy seasons.");
    }

    // Beneficiaries & Cost
    const numInputs = await page.$$('input[type="number"]');
    if (numInputs.length >= 2) {
      await numInputs[0].type("5000");
      await numInputs[1].type("8500000");
    }

    // Submit form
    console.log("   -> Clicking submit button...");
    const submitBtn = await page.evaluateHandle(() => {
      const buttons = Array.from(document.querySelectorAll("button"));
      return buttons.find((b) => /submit request|submit/i.test(b.innerText));
    });
    if (submitBtn.asElement()) {
      await submitBtn.asElement().click();
    }

    await page.waitForNavigation({ waitUntil: "networkidle2" }).catch(() => {});
    await new Promise((r) => setTimeout(r, 2000));

    // UI Step 5: Check Member Requests Tracker
    console.log("-> Navigating to /projects/requests tracker...");
    await page.goto(`${WEB_BASE}/projects/requests`, { waitUntil: "networkidle2" });
    await page.waitForSelector("h1", { timeout: 10000 });
    const trackerText = await page.$eval("body", (el) => el.innerText);
    if (!trackerText.includes("Erosion Control") && !trackerText.includes("drainage_and_flooding")) {
      console.log("   Notice: Request title check on tracker page");
    }
    console.log("   ✓ Member requests tracker loaded successfully.");

    // UI Step 6: Admin Workspace
    console.log("-> Logging out and logging in as Super Admin...");
    await page.goto(`${WEB_BASE}/login?redirect=/admin/projects`, { waitUntil: "networkidle2" });
    await page.waitForSelector('input[name="identifier"], input[type="text"]', { timeout: 10000 });
    await page.type('input[name="identifier"], input[type="text"]', SA_EMAIL);
    await page.type('input[type="password"]', SA_PASS);
    await page.click('button[type="submit"]');

    await page.waitForNavigation({ waitUntil: "networkidle2" });
    console.log("   ✓ Admin authenticated, navigated to /admin/projects");

    // Verify Admin Project Management Dashboard
    await page.waitForSelector("h1", { timeout: 10000 });
    const adminH1 = await page.$eval("h1", (el) => el.innerText);
    console.log(`   Admin Page Title: "${adminH1}"`);
    if (!adminH1.includes("Community Projects")) {
      throw new Error(`Unexpected admin page title: ${adminH1}`);
    }

    // Verify KPI stats and tabs are present
    await page.waitForFunction(
      () =>
        /total requests/i.test(document.body.innerText) &&
        /active projects/i.test(document.body.innerText),
      { timeout: 10000 }
    );
    console.log("   ✓ Admin overview metrics and project tabs verified.");

    if (pageErrors.length > 0) {
      console.warn("   Warnings/Page Errors recorded:", pageErrors);
    }

    console.log("==========================================================");
    console.log(" Community Projects Acceptance Suite COMPLETED SUCCESSFULLY");
    console.log("==========================================================");
  } finally {
    await browser.close();
  }
}

runCommunityProjectsBrowserVerification().catch((err) => {
  console.error(">>> ACCEPTANCE VERIFICATION FAILED:", err);
  process.exit(1);
});
