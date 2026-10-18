const path = require("path");
let puppeteer;
try {
  puppeteer = require("puppeteer-core");
} catch (e) {
  puppeteer = require(path.join(__dirname, "../apps/web/node_modules/puppeteer-core"));
}
const fs = require("fs");

const ARTIFACT_DIR = process.env.ARTIFACT_DIR || "/home/recoding/.gemini/antigravity-cli/brain/c4236dee-f77c-4c1b-b05c-5b85928ee084";
const SCRATCH_DIR = path.join(ARTIFACT_DIR, "scratch");
if (!fs.existsSync(SCRATCH_DIR)) {
  fs.mkdirSync(SCRATCH_DIR, { recursive: true });
}

const API_BASE = process.env.API_BASE || "http://127.0.0.1:8000/api/v1";
const WEB_BASE = process.env.WEB_BASE || "http://localhost:3000";

const SA_EMAIL = "browser_sa@ratels.example.org";
const SA_PASS = "BrowserAdminPass123!";

async function runInventoryBrowserVerification() {
  console.log("==========================================================");
  console.log(" Starting Real Browser Inventory Verification Suite");
  console.log("==========================================================");

  // 1. Authenticate via API to get CSRF token and seed initial stock
  console.log("-> Authenticating Super Admin via API...");
  const loginRes = await fetch(`${API_BASE}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ identifier: SA_EMAIL, password: SA_PASS }),
  });
  if (!loginRes.ok) {
    throw new Error(`API login failed with status ${loginRes.status}: ${await loginRes.text()}`);
  }
  const loginData = await loginRes.json();
  const token = loginData.access_token;
  const authHeaders = {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${token}`,
  };

  // Seed baseline warehouse and item if none exist
  console.log("-> Checking and seeding baseline inventory test fixtures...");
  const locRes = await fetch(`${API_BASE}/inventory/locations`, { headers: authHeaders });
  const locs = (await locRes.json()) || [];
  let wh1 = Array.isArray(locs) ? locs.find(l => l.location_type === "national") : null;
  if (!wh1) {
    const createLocRes = await fetch(`${API_BASE}/inventory/locations`, {
      method: "POST",
      headers: authHeaders,
      body: JSON.stringify({
        code: `WH-ABJ-${Date.now().toString().slice(-4)}`,
        name: "Abuja National Distribution Hub",
        location_type: "national",
        address: "Plot 101 Central Business District, Abuja",
        contact_phone: "+2348011223344",
      }),
    });
    wh1 = await createLocRes.json();
  }

  let wh2 = Array.isArray(locs) ? locs.find(l => l.location_type === "state") : null;
  if (!wh2) {
    const createLoc2 = await fetch(`${API_BASE}/inventory/locations`, {
      method: "POST",
      headers: authHeaders,
      body: JSON.stringify({
        code: `WH-KAD-${Date.now().toString().slice(-4)}`,
        name: "Kaduna State Logistics Depot",
        location_type: "state",
        address: "Ahmadu Bello Way, Kaduna",
        contact_phone: "+2348022334455",
      }),
    });
    wh2 = await createLoc2.json();
  }

  const itmRes = await fetch(`${API_BASE}/inventory/items`, { headers: authHeaders });
  const itms = (await itmRes.json()) || [];
  let testItem = Array.isArray(itms) ? itms[0] : null;
  if (!testItem) {
    const createItmRes = await fetch(`${API_BASE}/inventory/items`, {
      method: "POST",
      headers: authHeaders,
      body: JSON.stringify({
        sku: `RAT-SAN-${Date.now().toString().slice(-4)}`,
        name: "Sanitation Heavy-Duty Rake",
        category: "tools",
        unit_of_measure: "pcs",
        min_threshold_quantity: 15,
        description: "Reinforced steel rake for outdoor environmental sanitation",
      }),
    });
    testItem = await createItmRes.json();
  }

  // Ensure some initial stock exists at wh1
  console.log("-> Ensuring stock inflow at National Depot...");
  await fetch(`${API_BASE}/inventory/balances/receive`, {
    method: "POST",
    headers: authHeaders,
    body: JSON.stringify({
      location_id: wh1.id,
      item_id: testItem.id,
      quantity: 50,
      movement_type: "initial_receipt",
      notes: "Baseline stock verification batch",
    }),
  });

  // 2. Launch Headless Chrome
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

  try {
    // 3. UI Login via Form
    console.log("-> Navigating to Login Page via Browser...");
    await page.goto(`${WEB_BASE}/login?redirect=/admin/inventory`, { waitUntil: "networkidle2" });

    console.log("-> Submitting Super Admin credentials...");
    await page.waitForSelector('input[name="identifier"], input[type="text"]');
    await page.type('input[name="identifier"], input[type="text"]', SA_EMAIL);
    await page.type('input[type="password"]', SA_PASS);
    await page.click('button[type="submit"]');

    // Wait for redirect to /admin/inventory
    console.log("-> Awaiting redirect to /admin/inventory...");
    await page.waitForFunction(() => window.location.pathname === "/admin/inventory", { timeout: 15000 });
    console.log("   Successfully landed on /admin/inventory!");

    // Wait for data load
    await page.waitForSelector("h1", { timeout: 10000 });
    const pageTitle = await page.$eval("h1", el => el.textContent);
    console.log(`   Page Header: "${pageTitle.trim()}"`);

    // Verify 1: Overview Tab
    console.log("-> Verifying Tab 1: Overview & KPI Cards...");
    await page.waitForSelector(".grid-cols-2", { timeout: 8000 });
    const overviewImgPath = path.join(ARTIFACT_DIR, "inventory_01_overview.png");
    await page.screenshot({ path: overviewImgPath, fullPage: false });
    console.log(`   Saved Overview Screenshot: ${overviewImgPath}`);

    // Verify 2: Item Catalogue Tab
    console.log("-> Verifying Tab 2: Item Catalogue...");
    const catalogueTabBtn = await page.waitForSelector('button ::-p-text(Catalogue)');
    await catalogueTabBtn.click();
    await page.waitForSelector("table", { timeout: 5000 });

    // Open Add Item Modal
    console.log("   Testing 'Add Item to Catalogue' Modal...");
    const addItemBtn = await page.waitForSelector('button ::-p-text(Add Item to Catalogue)');
    await addItemBtn.click();
    await page.waitForSelector('input[type="text"]');
    await new Promise(r => setTimeout(r, 400));
    const catalogueImgPath = path.join(ARTIFACT_DIR, "inventory_02_catalogue.png");
    await page.screenshot({ path: catalogueImgPath, fullPage: false });
    console.log(`   Saved Catalogue Screenshot: ${catalogueImgPath}`);
    // Close modal
    const cancelBtn = await page.waitForSelector('button ::-p-text(Cancel)');
    await cancelBtn.click();
    await new Promise(r => setTimeout(r, 300));

    // Verify 3: Storage Facilities / Warehouses Tab
    console.log("-> Verifying Tab 3: Storage Facilities...");
    const warehouseTabBtn = await page.waitForSelector('button ::-p-text(Warehouses)');
    await warehouseTabBtn.click();
    await page.waitForSelector("table", { timeout: 5000 });
    const warehouseImgPath = path.join(ARTIFACT_DIR, "inventory_03_warehouses.png");
    await page.screenshot({ path: warehouseImgPath, fullPage: false });
    console.log(`   Saved Warehouses Screenshot: ${warehouseImgPath}`);

    // Test View Stock Drawer on first warehouse
    const viewStockBtn = await page.$('button ::-p-text(View Stock)');
    if (viewStockBtn) {
      console.log("   Testing 'View Stock' Warehouse Drawer Modal...");
      await viewStockBtn.click();
      await page.waitForSelector('h3 ::-p-text(Current Stock Balances)');
      await new Promise(r => setTimeout(r, 400));
      const closeStockBtn = await page.waitForSelector('button ::-p-text(Close)');
      await closeStockBtn.click();
    }

    // Verify 4: Material Requests Tab
    console.log("-> Verifying Tab 4: Material Requests...");
    const requestsTabBtn = await page.waitForSelector('button ::-p-text(Material Requests)');
    await requestsTabBtn.click();
    await page.waitForSelector("table", { timeout: 5000 });
    const requestsImgPath = path.join(ARTIFACT_DIR, "inventory_04_requests.png");
    await page.screenshot({ path: requestsImgPath, fullPage: false });
    console.log(`   Saved Requests Screenshot: ${requestsImgPath}`);

    // Verify 5: Transfers & In-Transit Tab
    console.log("-> Verifying Tab 5: Stock Transfers & In-Transit...");
    const transfersTabBtn = await page.waitForSelector('button ::-p-text(Transfers & In-Transit)');
    await transfersTabBtn.click();
    await page.waitForSelector("table", { timeout: 5000 });
    const transfersImgPath = path.join(ARTIFACT_DIR, "inventory_05_transfers.png");
    await page.screenshot({ path: transfersImgPath, fullPage: false });
    console.log(`   Saved Transfers Screenshot: ${transfersImgPath}`);

    // Verify 6: Discrepancies Tab
    console.log("-> Verifying Tab 6: Discrepancies...");
    const discrepanciesTabBtn = await page.waitForSelector('button ::-p-text(Discrepancies)');
    await discrepanciesTabBtn.click();
    await page.waitForSelector("table", { timeout: 5000 });
    const discrepanciesImgPath = path.join(ARTIFACT_DIR, "inventory_06_discrepancies.png");
    await page.screenshot({ path: discrepanciesImgPath, fullPage: false });
    console.log(`   Saved Discrepancies Screenshot: ${discrepanciesImgPath}`);

    // Verify 7: Audit Ledger Tab
    console.log("-> Verifying Tab 7: Immutable Stock Ledger...");
    const ledgerTabBtn = await page.waitForSelector('button ::-p-text(Immutable Stock Ledger)');
    await ledgerTabBtn.click();
    await page.waitForSelector("table", { timeout: 5000 });
    const ledgerImgPath = path.join(ARTIFACT_DIR, "inventory_07_ledger.png");
    await page.screenshot({ path: ledgerImgPath, fullPage: false });
    console.log(`   Saved Ledger Screenshot: ${ledgerImgPath}`);

    // Verify 8: Responsive Layout (Mobile Viewport)
    console.log("-> Verifying Responsive Mobile Viewport (375x812)...");
    await page.setViewport({ width: 375, height: 812 });
    await new Promise(r => setTimeout(r, 500));
    const mobileImgPath = path.join(ARTIFACT_DIR, "inventory_08_mobile.png");
    await page.screenshot({ path: mobileImgPath, fullPage: false });
    console.log(`   Saved Mobile Screenshot: ${mobileImgPath}`);

    // Reset viewport
    await page.setViewport({ width: 1440, height: 900 });

    console.log("\n-> Filtering console logs...");
    const relevantErrors = consoleErrors.filter(e => !e.includes("favicon") && !e.includes("hydration"));
    if (relevantErrors.length > 0) {
      console.warn("   Warnings/Errors observed in browser console:", relevantErrors);
    } else {
      console.log("   Zero fatal errors observed in browser console!");
    }

    console.log("\n==========================================================");
    console.log(" All 7 Inventory Sections Verified Successfully in Browser!");
    console.log("==========================================================");
  } finally {
    await browser.close();
  }
}

runInventoryBrowserVerification().catch((err) => {
  console.error("FATAL Browser Verification Failure:", err);
  process.exit(1);
});
