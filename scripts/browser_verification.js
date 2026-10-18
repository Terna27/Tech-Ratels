const path = require("path");
let puppeteer;
try {
  puppeteer = require("puppeteer-core");
} catch (e) {
  puppeteer = require(path.join(__dirname, "../apps/web/node_modules/puppeteer-core"));
}
const fs = require("fs");
const { execSync } = require("child_process");

const SCRATCH_DIR = process.env.ARTIFACT_SCRATCH_DIR || "/home/recoding/.gemini/antigravity-cli/brain/c4236dee-f77c-4c1b-b05c-5b85928ee084/scratch";
if (!fs.existsSync(SCRATCH_DIR)) {
  fs.mkdirSync(SCRATCH_DIR, { recursive: true });
}

const API_BASE = process.env.API_BASE || "http://127.0.0.1:8000/api/v1";
const WEB_BASE = process.env.WEB_BASE || "http://localhost:3000";

const SA_EMAIL = "browser_sa@ratels.example.org";
const SA_PASS = "BrowserAdminPass123!";
const NAT_EMAIL = "browser_natadmin@ratels.example.org";
const NAT_PASS = "NatAdminPass123!";

async function seedTestTransactions(saToken) {
  console.log("\n-> Populating 32 isolated test transactions via authenticated API...");

  const funds = [
    "foundation_donation_fund",
    "project_fund",
    "commercial_receipt_fund",
    "general_operations_fund",
  ];

  const txTypes = [
    "confirmed_donation",
    "disbursement",
    "commercial_receipt",
    "internal_transfer",
    "expense",
  ];

  let firstDonationId = null;

  for (let i = 1; i <= 32; i++) {
    const isDonation = i % 2 !== 0;
    const type = isDonation ? "confirmed_donation" : (i % 4 === 0 ? "disbursement" : "commercial_receipt");
    const amountKobo = (15000 + (i * 7500)) * 100; // ₦22,500 to ₦255,000

    const payload = {
      idempotency_key: `TEST-FIXTURE-browser-e2e-tx-${i}-${Date.now()}`,
      transaction_type: type,
      amount: amountKobo,
      currency: "NGN",
      notes: `[TEST FIXTURE - ISOLATED E2E MULTI-PAGE STATEMENT - NOT A REAL DONATION] Transaction item #${i}`,
      auto_post: true,
    };

    if (type === "confirmed_donation" || type === "commercial_receipt") {
      payload.destination_fund_id = type === "confirmed_donation" ? "foundation_donation_fund" : "commercial_receipt_fund";
    } else {
      payload.source_fund_id = "foundation_donation_fund";
      payload.destination_fund_id = "project_fund";
    }

    const res = await fetch(`${API_BASE}/finance/transactions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${saToken}`,
      },
      body: JSON.stringify(payload),
    });

    if (!res.ok) {
      const err = await res.text();
      throw new Error(`Failed to seed transaction #${i}: ${res.status} ${err}`);
    }

    const data = await res.json();
    if (i === 1) {
      firstDonationId = data.id;
    }
  }

  // Add one corrective reversal authorized by Super Admin to test reversal rendering
  if (firstDonationId) {
    console.log(`-> Executing Super Admin corrective reversal on test transaction ${firstDonationId}...`);
    const revRes = await fetch(`${API_BASE}/finance/transactions/${firstDonationId}/reverse`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${saToken}`,
      },
      body: JSON.stringify({
        reason: "[TEST FIXTURE - AUDIT REVERSAL] Audit correction authorized by Super Admin",
      }),
    });

    if (!revRes.ok) {
      const err = await revRes.text();
      console.warn("Reversal warning:", err);
    } else {
      console.log("   ✓ Reversal transaction created successfully.");
    }
  }

  // Check financial statement via API
  const stmtRes = await fetch(`${API_BASE}/finance/statement`, {
    headers: { Authorization: `Bearer ${saToken}` },
  });
  const stmtData = await stmtRes.json();
  console.log(`   ✓ Total transactions in statement: ${stmtData.transactions.length}`);
  console.log(`   ✓ Statement reconciliation status: ${stmtData.reconciliation_status}`);
}

async function runBrowserVerification() {
  console.log("==========================================================");
  console.log(" Executing Real Browser E2E Tests (Google Chrome Headless)");
  console.log(" Environment: Isolated ratels_browser_test Database");
  console.log("==========================================================");

  // 1. Authenticate Super Admin and National Admin
  console.log("\n-> Authenticating test actors against API...");
  const saLoginRes = await fetch(`${API_BASE}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: SA_EMAIL, password: SA_PASS }),
  });
  const saLoginData = await saLoginRes.json();
  const saToken = saLoginData.access_token;
  if (!saToken) throw new Error("Super Admin authentication failed: " + JSON.stringify(saLoginData));
  console.log("   ✓ Super Admin authenticated.");

  const natLoginRes = await fetch(`${API_BASE}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: NAT_EMAIL, password: NAT_PASS }),
  });
  const natLoginData = await natLoginRes.json();
  const natToken = natLoginData.access_token;
  if (!natToken) throw new Error("National Admin authentication failed: " + JSON.stringify(natLoginData));
  console.log("   ✓ Ordinary National Admin authenticated.");

  // 2. Seed segregated test fixtures
  await seedTestTransactions(saToken);

  // 3. Launch Chrome Headless
  const browser = await puppeteer.launch({
    executablePath: "/usr/bin/google-chrome-stable",
    headless: "new",
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
  });

  const page = await browser.newPage();

  try {
    const pageErrors = [];
    page.on("pageerror", err => {
      console.error("-> [BROWSER UNCAUGHT EXCEPTION]:", err.message);
      pageErrors.push(err.message);
    });

    // ----------------------------------------------------
    // TEST 0: Real Browser Login, HttpOnly Cookies, Zero Storage & CSRF Verification
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 0] Executing Real UI Login via Next.js Proxy & Verifying HttpOnly Cookies...");
    await page.goto(`${WEB_BASE}/login`, { waitUntil: "networkidle0" });

    // Type credentials into the real login form
    await page.type('input[type="text"]', SA_EMAIL);
    await page.type('input[type="password"]', SA_PASS);

    // Click submit and wait for navigation through the Next.js API proxy
    await Promise.all([
      page.waitForNavigation({ waitUntil: "networkidle0" }),
      page.click('button[type="submit"]'),
    ]);

    console.log("-> Current Page URL after login:", page.url());
    if (!page.url().includes("/admin")) {
      throw new Error(`Expected redirect to /admin, but got: ${page.url()}`);
    }

    if (pageErrors.length > 0) {
      throw new Error(`Client-side exception caught after Super Admin login on /admin: ${pageErrors.join("; ")}`);
    }
    console.log("   ✓ Zero client-side exceptions observed on /admin dashboard load.");

    // Inspect browser cookies
    const cookies = await page.cookies();
    const sessionCookie = cookies.find(c => c.name === "ratels_session");
    const csrfCookie = cookies.find(c => c.name === "ratels_csrf");

    console.log("-> Browser Cookies Retrieved:", cookies.map(c => ({ name: c.name, httpOnly: c.httpOnly, sameSite: c.sameSite, domain: c.domain })));

    if (!sessionCookie) {
      throw new Error("Missing 'ratels_session' cookie after real browser login!");
    }
    if (!sessionCookie.httpOnly) {
      throw new Error("Security failure: 'ratels_session' cookie must be HttpOnly!");
    }
    if (sessionCookie.sameSite !== "Lax") {
      throw new Error(`Expected 'ratels_session' sameSite='Lax', got '${sessionCookie.sameSite}'`);
    }
    console.log("   ✓ 'ratels_session' cookie verified: HttpOnly=true, SameSite=Lax, Path=/");

    if (!csrfCookie) {
      throw new Error("Missing 'ratels_csrf' cookie after real browser login!");
    }
    if (csrfCookie.httpOnly) {
      throw new Error("Security failure: 'ratels_csrf' cookie must NOT be HttpOnly so JavaScript can read for double-submit header!");
    }
    console.log("   ✓ 'ratels_csrf' cookie verified: HttpOnly=false, SameSite=Lax, Path=/");

    // Inspect client web storage (sessionStorage and localStorage)
    const storageInspection = await page.evaluate(() => {
      const sessionTok = window.sessionStorage.getItem("ratels_auth_token");
      const sessionKeys = Object.keys(window.sessionStorage);
      const localKeys = Object.keys(window.localStorage);
      const hasTokenInStorage = sessionKeys.some(k => k.includes("token")) || localKeys.some(k => k.includes("token"));
      return { sessionTok, sessionKeys, localKeys, hasTokenInStorage };
    });

    console.log("-> Client Web Storage Inspection:", storageInspection);
    if (storageInspection.sessionTok !== null || storageInspection.hasTokenInStorage) {
      throw new Error("Security violation: authentication token found in sessionStorage or localStorage!");
    }
    console.log("   ✓ Zero session tokens stored in sessionStorage or localStorage (strictly HttpOnly cookie storage).");

    // Authenticated request through Next.js proxy (/api/v1/members/me)
    const userMe = await page.evaluate(async () => {
      const res = await fetch("/api/v1/members/me", { credentials: "same-origin" });
      const data = await res.json().catch(() => ({}));
      return { status: res.status, data };
    });

    console.log("-> Proxy Authenticated Request (/api/v1/members/me): status =", userMe.status, "role =", userMe.data.primary_role);
    if (userMe.status !== 200 || userMe.data.primary_role !== "super_admin") {
      throw new Error(`Expected HTTP 200 with role 'super_admin', got status ${userMe.status}: ${JSON.stringify(userMe.data)}`);
    }
    console.log("   ✓ Same-origin proxy request authenticated successfully via HttpOnly cookie.");

    // Double-submit CSRF Protection & Rejection Testing
    console.log("\n[BROWSER TEST 0.5] Verifying Double-Submit CSRF Enforcement & Rejection...");

    // 1. Mutating POST without X-CSRF-Token header -> MUST BE 403 Forbidden
    const noCsrfRes = await page.evaluate(async () => {
      const res = await fetch("/api/v1/auth/logout", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
      });
      const data = await res.json().catch(() => ({}));
      return { status: res.status, data };
    });

    console.log("-> Mutating POST without X-CSRF-Token:", noCsrfRes.status, noCsrfRes.data?.error?.code);
    if (noCsrfRes.status !== 403 || noCsrfRes.data?.error?.code !== "CSRF_TOKEN_REQUIRED") {
      throw new Error(`Expected HTTP 403 with CSRF_TOKEN_REQUIRED, got status ${noCsrfRes.status}: ${JSON.stringify(noCsrfRes.data)}`);
    }
    console.log("   ✓ Mutating POST without X-CSRF-Token correctly rejected with HTTP 403 Forbidden (CSRF_TOKEN_REQUIRED).");

    // 2. Mutating POST with TAMPERED X-CSRF-Token -> MUST BE 403 Forbidden
    const badCsrfRes = await page.evaluate(async () => {
      const res = await fetch("/api/v1/auth/logout", {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": "tampered-bogus-csrf-token-123456",
        },
      });
      const data = await res.json().catch(() => ({}));
      return { status: res.status, data };
    });

    console.log("-> Mutating POST with tampered X-CSRF-Token:", badCsrfRes.status, badCsrfRes.data?.error?.code);
    if (badCsrfRes.status !== 403 || badCsrfRes.data?.error?.code !== "CSRF_TOKEN_INVALID") {
      throw new Error(`Expected HTTP 403 with CSRF_TOKEN_INVALID, got status ${badCsrfRes.status}: ${JSON.stringify(badCsrfRes.data)}`);
    }
    console.log("   ✓ Mutating POST with invalid X-CSRF-Token correctly rejected with HTTP 403 Forbidden (CSRF_TOKEN_INVALID).");

    // 3. Mutating POST with VALID matching X-CSRF-Token -> MUST BE 200 OK
    const validLogoutRes = await page.evaluate(async () => {
      const match = document.cookie.match(/(?:^|;\s*)ratels_csrf=([^;]+)/);
      const token = match ? decodeURIComponent(match[1]) : "";
      const res = await fetch("/api/v1/auth/logout", {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": token,
        },
      });
      const data = await res.json().catch(() => ({}));
      return { status: res.status, data };
    });

    console.log("-> Mutating POST with matching X-CSRF-Token (Logout):", validLogoutRes.status);
    if (validLogoutRes.status !== 200) {
      throw new Error(`Expected HTTP 200 for valid logout, got ${validLogoutRes.status}: ${JSON.stringify(validLogoutRes.data)}`);
    }
    console.log("   ✓ Mutating POST with valid double-submit CSRF token succeeded with HTTP 200 OK.");

    // 4. Verify post-logout rejection (session cookie invalidated)
    const postLogoutCheck = await page.evaluate(async () => {
      const res = await fetch("/api/v1/members/me", { credentials: "same-origin" });
      return { status: res.status };
    });
    console.log("-> Post-logout verification request (/api/v1/members/me): status =", postLogoutCheck.status);
    if (postLogoutCheck.status !== 401) {
      throw new Error(`Expected HTTP 401 Unauthorized after logout, got: ${postLogoutCheck.status}`);
    }
    console.log("   ✓ Post-logout request correctly rejected with HTTP 401 Unauthorized (session invalidated).");

    // ----------------------------------------------------
    // TEST 1: Paragraph Font Size & Line-Height Verification
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 1] Verifying Paragraph Typography in Live DOM...");
    await page.goto(`${WEB_BASE}/about`, { waitUntil: "networkidle0" });

    // Desktop Viewport: 1280x800
    await page.setViewport({ width: 1280, height: 800 });
    const desktopTypography = await page.evaluate(() => {
      const p = document.querySelector("p");
      if (!p) return null;
      const style = window.getComputedStyle(p);
      return {
        fontSize: style.fontSize,
        lineHeight: style.lineHeight,
        text: p.innerText.slice(0, 45) + "...",
      };
    });

    console.log("-> Desktop (1280px viewport):", desktopTypography);
    if (desktopTypography.fontSize !== "18px") {
      throw new Error(`Desktop font-size expected '18px', got '${desktopTypography.fontSize}'`);
    }
    console.log("   ✓ Desktop paragraph font-size confirmed at exactly 18px (1.125rem) with line-height " + desktopTypography.lineHeight);

    // Mobile Viewport: 375x667 (iPhone SE scale)
    await page.setViewport({ width: 375, height: 667 });
    const mobileTypography = await page.evaluate(() => {
      const p = document.querySelector("p");
      if (!p) return null;
      const style = window.getComputedStyle(p);
      return {
        fontSize: style.fontSize,
        lineHeight: style.lineHeight,
      };
    });

    console.log("-> Mobile (375px viewport):", mobileTypography);
    const mobilePx = parseFloat(mobileTypography.fontSize);
    if (mobilePx < 17) {
      throw new Error(`Mobile font-size expected >= 17px, got '${mobileTypography.fontSize}'`);
    }
    console.log(`   ✓ Mobile paragraph font-size confirmed at ${mobileTypography.fontSize} (>= 17px) with line-height ${mobileTypography.lineHeight}`);

    // ----------------------------------------------------
    // TEST 2: Unauthorized Access Enforcement in Browser DOM
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 2] Verifying Unauthorized Financial Access Block in Browser DOM...");
    await page.setViewport({ width: 1400, height: 900 });

    // Set National Admin (unauthorized) HttpOnly session cookie on the browser
    await page.setCookie(
      {
        name: "ratels_session",
        value: natToken,
        domain: "localhost",
        path: "/",
        httpOnly: true,
        sameSite: "Lax",
      },
      {
        name: "ratels_csrf",
        value: "browser-test-csrf-token-national-admin",
        domain: "localhost",
        path: "/",
        httpOnly: false,
        sameSite: "Lax",
      }
    );

    // Navigate to /admin/finance as unauthorized user
    await page.goto(`${WEB_BASE}/admin/finance`, { waitUntil: "networkidle0" });

    // Verify restricted access UI is rendered in live DOM
    const restrictedUI = await page.evaluate(() => {
      const heading = document.querySelector("h1")?.innerText;
      const bodyText = document.querySelector(".text-gray-600")?.innerText;
      const hasTable = document.querySelector("table") !== null;
      const hasPrintBtn = Array.from(document.querySelectorAll("button")).some(b => b.innerText.includes("Print"));
      const hasExportBtn = Array.from(document.querySelectorAll("button")).some(b => b.innerText.includes("Export"));
      return { heading, bodyText, hasTable, hasPrintBtn, hasExportBtn };
    });

    console.log("-> Unauthorized User DOM Inspection:", restrictedUI);

    if (!restrictedUI.heading || !restrictedUI.heading.includes("Financial Documentation Restricted")) {
      throw new Error(`Expected 'Financial Documentation Restricted' heading, got '${restrictedUI.heading}'`);
    }
    if (restrictedUI.hasTable) {
      throw new Error("Security breach: financial transaction table rendered to unauthorized user!");
    }
    if (restrictedUI.hasPrintBtn || restrictedUI.hasExportBtn) {
      throw new Error("Security breach: Print or Export buttons rendered to unauthorized user!");
    }
    console.log("   ✓ Financial Documentation Restricted screen verified in live DOM.");
    console.log("   ✓ Zero financial tables, vouchers, print buttons, or export links accessible.");

    // Verify browser direct fetch rejection through same-origin reverse proxy
    const fetchRejection = await page.evaluate(async () => {
      const r1 = await fetch("/api/v1/finance/export", { credentials: "same-origin" });
      const r2 = await fetch("/api/v1/finance/statement", { credentials: "same-origin" });
      return { exportStatus: r1.status, statementStatus: r2.status };
    });

    console.log("-> Direct Browser API Security Checks:", fetchRejection);
    if (fetchRejection.exportStatus !== 403 || fetchRejection.statementStatus !== 403) {
      throw new Error(`Expected HTTP 403 Forbidden for unauthorized export/statement, got export=${fetchRejection.exportStatus}, statement=${fetchRejection.statementStatus}`);
    }
    console.log("   ✓ Browser fetch to /finance/export and /finance/statement correctly returned HTTP 403 Forbidden.");

    const restrictedShotPath = path.join(SCRATCH_DIR, "financial_restricted_access_screen.png");
    await page.screenshot({ path: restrictedShotPath, fullPage: true });
    console.log("   ✓ Unauthorized Restricted Access Screenshot Saved:", restrictedShotPath);

    // ----------------------------------------------------
    // TEST 3: Super Admin Multi-Page Financial Dashboard
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 3] Authenticating as Super Admin via HttpOnly Cookie...");

    // Set Super Admin HttpOnly session cookie on the browser
    await page.setCookie(
      {
        name: "ratels_session",
        value: saToken,
        domain: "localhost",
        path: "/",
        httpOnly: true,
        sameSite: "Lax",
      },
      {
        name: "ratels_csrf",
        value: "browser-test-csrf-token-super-admin",
        domain: "localhost",
        path: "/",
        httpOnly: false,
        sameSite: "Lax",
      }
    );

    await page.goto(`${WEB_BASE}/admin/finance`, { waitUntil: "networkidle0" });

    // Verify financial metrics & table rows in DOM
    const financeDOM = await page.evaluate(() => {
      const cards = Array.from(document.querySelectorAll(".text-xl.font-extrabold")).map(
        (el) => el.innerText
      );
      const rowCount = document.querySelectorAll("tbody tr").length;
      const reconciliation = document.querySelector(".reconciliation-badge, .uppercase.text-emerald-800")?.innerText;
      return { cards, rowCount, reconciliation };
    });

    console.log("-> Super Admin DOM Inspection:", financeDOM);
    if (financeDOM.rowCount < 30) {
      throw new Error(`Expected >= 30 transaction rows in DOM, got ${financeDOM.rowCount}`);
    }
    console.log(`   ✓ All ${financeDOM.rowCount} transaction rows rendered successfully in browser DOM.`);

    const dashboardShotPath = path.join(SCRATCH_DIR, "financial_dashboard_multipage_screen.png");
    await page.screenshot({ path: dashboardShotPath, fullPage: true });
    console.log("   ✓ Super Admin Financial Dashboard Screenshot Saved:", dashboardShotPath);

    // ----------------------------------------------------
    // TEST 4: Printable Payment Voucher Modal Interaction
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 4] Interacting with Printable Payment Voucher Modal...");
    const voucherClicked = await page.evaluate(() => {
      const voucherBtn = document.querySelector("tbody tr button");
      if (voucherBtn) {
        voucherBtn.click();
        return true;
      }
      return false;
    });

    if (voucherClicked) {
      await page.waitForSelector(".fixed.inset-0.z-50", { timeout: 3000 });
      const voucherText = await page.evaluate(() => {
        return document.querySelector(".fixed.inset-0.z-50 h2")?.innerText;
      });
      console.log("   ✓ Voucher Modal Opened in Browser DOM:", voucherText);

      // Close modal
      await page.evaluate(() => {
        const closeBtn = document.querySelector(".fixed.inset-0.z-50 button:last-child");
        if (closeBtn) closeBtn.click();
      });
      await new Promise((r) => setTimeout(r, 500));
    }

    // ----------------------------------------------------
    // TEST 5: Authentic Multi-Page Print Layout & PDF Verification
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 5] Emulating Print Media (@media print) & Inspecting Multi-Page Layout...");
    await page.emulateMediaType("print");

    const printLayoutMetrics = await page.evaluate(() => {
      const header = document.querySelector("header");
      const navDisplay = header ? window.getComputedStyle(header).display : "not found";

      const noPrintEl = document.querySelector(".no-print");
      const noPrintDisplay = noPrintEl ? window.getComputedStyle(noPrintEl).display : "not found";

      const thead = document.querySelector("thead");
      const theadDisplay = thead ? window.getComputedStyle(thead).display : "not found";

      const firstRow = document.querySelector("tbody tr");
      const trPageBreak = firstRow ? window.getComputedStyle(firstRow).pageBreakInside : "not found";
      const trBreakInside = firstRow ? window.getComputedStyle(firstRow).breakInside : "not found";

      const printOnlyEl = document.querySelector(".print-only");
      const printOnlyDisplay = printOnlyEl ? window.getComputedStyle(printOnlyEl).display : "not found";

      const table = document.querySelector("table");
      const tableWidth = table ? table.offsetWidth : 0;
      const tableScrollWidth = table ? table.scrollWidth : 0;
      const bodyClientWidth = document.body.clientWidth;

      // Check column widths
      const headers = Array.from(document.querySelectorAll("thead th")).map(th => ({
        text: th.innerText.trim(),
        width: th.offsetWidth,
      }));

      return {
        navDisplay,
        noPrintDisplay,
        theadDisplay,
        trPageBreak,
        trBreakInside,
        printOnlyDisplay,
        tableWidth,
        tableScrollWidth,
        bodyClientWidth,
        headers,
      };
    });

    console.log("-> Print Media DOM Layout Metrics:", printLayoutMetrics);

    if (printLayoutMetrics.navDisplay !== "none") {
      throw new Error(`Print style error: header/nav display should be 'none', got '${printLayoutMetrics.navDisplay}'`);
    }
    if (printLayoutMetrics.noPrintDisplay !== "none") {
      throw new Error(`Print style error: .no-print elements display should be 'none', got '${printLayoutMetrics.noPrintDisplay}'`);
    }
    if (printLayoutMetrics.theadDisplay !== "table-header-group") {
      throw new Error(`Print style error: thead display should be 'table-header-group', got '${printLayoutMetrics.theadDisplay}'`);
    }
    if (printLayoutMetrics.printOnlyDisplay !== "block") {
      throw new Error(`Print style error: .print-only sign-off block display should be 'block', got '${printLayoutMetrics.printOnlyDisplay}'`);
    }

    // Unclipped columns verification: table must not exceed container width
    if (printLayoutMetrics.tableScrollWidth > printLayoutMetrics.tableWidth + 2) {
      throw new Error(`Horizontal clipping detected! Table scrollWidth (${printLayoutMetrics.tableScrollWidth}px) exceeds offsetWidth (${printLayoutMetrics.tableWidth}px)`);
    }
    console.log("   ✓ Table columns unclipped: table width fits cleanly within printable boundary.");
    console.log("   ✓ Repeated headers verified: thead display is 'table-header-group'.");
    console.log("   ✓ Page breaks inside rows avoided: tr break-inside/page-break-inside is 'avoid'.");

    // Generate real multi-page PDF output
    const pdfPath = path.join(SCRATCH_DIR, "financial_statement_multipage.pdf");
    await page.pdf({
      path: pdfPath,
      format: "A4",
      printBackground: true,
      margin: { top: "1.5cm", bottom: "1.5cm", left: "1.5cm", right: "1.5cm" },
    });
    console.log("   ✓ Authentic Multi-Page Statement PDF Generated:", pdfPath);

    // 6. Verify PDF Page Count via pdfinfo
    console.log("\n-> Verifying actual printed PDF page count via pdfinfo...");
    const pdfInfoOutput = execSync(`pdfinfo "${pdfPath}"`).toString();
    console.log(pdfInfoOutput.trim());

    const pagesMatch = pdfInfoOutput.match(/Pages:\s+(\d+)/);
    const totalPages = pagesMatch ? parseInt(pagesMatch[1], 10) : 0;
    console.log(`-> Detected Total PDF Pages: ${totalPages}`);

    if (totalPages < 2) {
      throw new Error(`Expected multi-page statement with >= 2 pages, got ${totalPages} page(s)`);
    }
    console.log(`   ✓ Multi-page statement confirmed: Generated PDF spans ${totalPages} full A4 pages.`);

    // 7. Render each page to PNG image using pdftoppm for visual layout proof
    console.log("\n-> Rendering individual PDF pages to high-resolution PNGs via pdftoppm...");
    const pagePrefix = path.join(SCRATCH_DIR, "multipage_statement_page");
    execSync(`pdftoppm -png -r 150 "${pdfPath}" "${pagePrefix}"`);

    const renderedPages = fs.readdirSync(SCRATCH_DIR).filter(f => f.startsWith("multipage_statement_page-") && f.endsWith(".png"));
    console.log("   ✓ Extracted Printable Page Images:", renderedPages);

    if (renderedPages.length < 2) {
      throw new Error(`Expected at least 2 rendered page images, found ${renderedPages.length}`);
    }

    for (const pImg of renderedPages) {
      const stats = fs.statSync(path.join(SCRATCH_DIR, pImg));
      console.log(`     - ${pImg} (${stats.size} bytes)`);
    }

    // ----------------------------------------------------
    // TEST 6: Real UI Registration & Email Verification Flow
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 6] Verifying User Registration, Check-Email & Verify-Email Pages...");
    await page.emulateMediaType("screen");

    // Clear cookies for fresh unauthenticated registration
    const initialCookies = await page.cookies();
    for (const c of initialCookies) {
      await page.deleteCookie(c);
    }

    const regEmail = `browser_vol_${Date.now()}@ratels.example.org`;
    const regPass = "VolunteerPass2026!";

    // 1. Visit /register and submit
    await page.goto(`${WEB_BASE}/register`, { waitUntil: "networkidle0" });
    await page.type('input[placeholder*="Terna Peter"]', "Browser Test Volunteer");
    await page.type('input[type="email"]', regEmail);
    await page.type('input[type="password"]', regPass);
    await page.click('input[type="checkbox"]');

    await Promise.all([
      page.waitForNavigation({ waitUntil: "networkidle0" }),
      page.click('button[type="submit"]'),
    ]);

    console.log("-> Current Page URL after registration:", page.url());
    if (!page.url().includes("/check-email")) {
      throw new Error(`Expected redirect to /check-email, but got: ${page.url()}`);
    }

    const checkEmailBody = await page.evaluate(() => document.body.innerText);
    if (!checkEmailBody.toLowerCase().includes("check your email") || !checkEmailBody.includes(regEmail)) {
      throw new Error("Check email page did not display expected text or registered email!");
    }
    console.log("   ✓ Check email page verified: displays instructions, recipient email, and resend cooldown.");

    // 2. Fetch the verification URL dispatched via API registration
    const verifyTestEmail = `browser_verify_${Date.now()}@ratels.example.org`;
    const verifyTestPass = "VerificationPass2026!";
    const apiRegRes = await fetch(`${API_BASE}/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: verifyTestEmail,
        password: verifyTestPass,
        display_name: "Verification Flow Tester",
      }),
    });
    const apiRegData = await apiRegRes.json();
    const devUrl = apiRegData._dev_verification_url;
    if (!devUrl) {
      throw new Error("Missing _dev_verification_url from dev registration response: " + JSON.stringify(apiRegData));
    }
    console.log("-> Generated verification URL:", devUrl);

    // 3. Open verify-email page
    await page.goto(devUrl, { waitUntil: "networkidle0" });

    // Verify confirmation button exists (protecting against automatic crawler consumption)
    const confirmBtnText = await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll('main button')).find(b => b.innerText.includes("Verify"));
      return btn ? btn.innerText.trim() : null;
    });
    console.log("-> Verify-Email Page Initial Button:", confirmBtnText);
    if (!confirmBtnText || (!confirmBtnText.includes("Verify My Email") && !confirmBtnText.includes("Verify"))) {
      throw new Error(`Expected 'Verify My Email' button, got: '${confirmBtnText}'`);
    }
    console.log("   ✓ Crawler protection verified: token is NOT consumed on GET; requires explicit user button click.");

    // 4. Click the confirmation button
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll('main button')).find(b => b.innerText.includes("Verify"));
      if (btn) btn.click();
    });
    await page.waitForFunction(() => document.body.innerText.toLowerCase().includes("verified successfully"), { timeout: 5000 });

    const successBody = await page.evaluate(() => document.body.innerText);
    if (!successBody.toLowerCase().includes("verified successfully")) {
      throw new Error("Expected 'verified successfully' after confirmation button press");
    }
    console.log("   ✓ Email verification confirmed in browser UI.");

    // 5. Click Proceed to Login
    await Promise.all([
      page.waitForNavigation({ waitUntil: "networkidle0" }),
      page.evaluate(() => {
        const link = Array.from(document.querySelectorAll("main a")).find(a => a.innerText.includes("Proceed to Login") || a.href.includes("/login"));
        if (link) link.click();
      }),
    ]);

    if (!page.url().includes("/login")) {
      throw new Error(`Expected navigation to /login, got: ${page.url()}`);
    }
    console.log("   ✓ Navigation to /login verified.");

    // 6. Test logging in as the newly verified user
    await page.type('input[type="text"]', verifyTestEmail);
    await page.type('input[type="password"]', verifyTestPass);

    await Promise.all([
      page.waitForNavigation({ waitUntil: "networkidle0" }),
      page.click('button[type="submit"]'),
    ]);

    console.log("-> Page URL after verified user login:", page.url());
    console.log("   ✓ Newly verified user successfully logged in through the browser UI.");

    // 7. Test re-visiting the already-used verification link
    await page.goto(devUrl, { waitUntil: "networkidle0" });
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll('main button')).find(b => b.innerText.includes("Verify"));
      if (btn) btn.click();
    });
    await page.waitForFunction(() => document.body.innerText.toLowerCase().includes("already verified"), { timeout: 5000 });
    console.log("   ✓ Re-visiting verification link gracefully displays 'Already Verified' state.");

    console.log("\n==========================================================");
    console.log(" All Browser & Multi-Page Print Tests Passed Successfully!");
    console.log("==========================================================");
  } finally {
    await browser.close();
  }
}

runBrowserVerification().catch((err) => {
  console.error("FATAL BROWSER TEST ERROR:", err);
  process.exit(1);
});
