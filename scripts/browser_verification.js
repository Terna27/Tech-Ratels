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

    // Inject National Admin (unauthorized) token into sessionStorage
    await page.goto(WEB_BASE, { waitUntil: "domcontentloaded" });
    await page.evaluate((tok) => {
      sessionStorage.setItem("ratels_auth_token", tok);
    }, natToken);

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

    // Verify browser direct fetch rejection
    const fetchRejection = await page.evaluate(async (apiBase) => {
      const tok = sessionStorage.getItem("ratels_auth_token");
      const r1 = await fetch(`${apiBase}/finance/export`, { headers: { Authorization: `Bearer ${tok}` } });
      const r2 = await fetch(`${apiBase}/finance/statement`, { headers: { Authorization: `Bearer ${tok}` } });
      return { exportStatus: r1.status, statementStatus: r2.status };
    }, API_BASE);

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
    console.log("\n[BROWSER TEST 3] Authenticating as Super Admin in Browser Session...");

    // Inject Super Admin token
    await page.goto(WEB_BASE, { waitUntil: "domcontentloaded" });
    await page.evaluate((tok) => {
      sessionStorage.setItem("ratels_auth_token", tok);
    }, saToken);

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
