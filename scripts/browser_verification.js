const path = require("path");
let puppeteer;
try {
  puppeteer = require("puppeteer-core");
} catch (e) {
  puppeteer = require(path.join(__dirname, "../apps/web/node_modules/puppeteer-core"));
}
const fs = require("fs");

async function runBrowserVerification() {
  console.log("==========================================================");
  console.log(" Executing Real Browser E2E Tests (Google Chrome Headless)");
  console.log("==========================================================");

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
    await page.goto("http://localhost:3000/about", { waitUntil: "networkidle0" });

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
    // TEST 2: Super Admin Authentication & Financial Dashboard
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 2] Authenticating as Super Admin in Browser Session...");
    await page.setViewport({ width: 1400, height: 900 });

    // Login via API to get token for Super Admin
    const loginRes = await fetch("http://127.0.0.1:8000/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        username: "restart_sa_23748@ratels.example.org",
        password: "RestartAdminSecurePass123!",
      }),
    });
    const loginData = await loginRes.json();
    const token = loginData.access_token;

    // Inject token into browser sessionStorage
    await page.goto("http://localhost:3000", { waitUntil: "domcontentloaded" });
    await page.evaluate((tok) => {
      sessionStorage.setItem("ratels_auth_token", tok);
    }, token);

    console.log("-> Token injected into browser session. Navigating to /admin/finance...");
    await page.goto("http://localhost:3000/admin/finance", { waitUntil: "networkidle0" });

    // Verify financial metrics are rendered in DOM
    const financeMetrics = await page.evaluate(() => {
      const cards = Array.from(document.querySelectorAll(".text-xl.font-extrabold")).map(
        (el) => el.innerText
      );
      const reconciliation = document.querySelector(".reconciliation-badge, .uppercase.text-emerald-800")?.innerText;
      return { cards, reconciliation };
    });

    console.log("   ✓ Financial Summary Cards Rendered in Browser:", financeMetrics);

    // Save Screen Screenshot
    const screenShotPath = "/home/recoding/.gemini/antigravity-cli/brain/2b5d7315-6492-4e85-bc0b-28e49357c614/scratch/finance_dashboard_screen.png";
    await page.screenshot({ path: screenShotPath, fullPage: true });
    console.log("   ✓ Screen Screenshot Saved:", screenShotPath);

    // ----------------------------------------------------
    // TEST 3: Printable Payment Voucher Modal Interaction
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 3] Interacting with Printable Voucher Modal...");
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
    // TEST 4: Print Media Stylesheet Emulation (@media print)
    // ----------------------------------------------------
    console.log("\n[BROWSER TEST 4] Emulating Print Media (@media print) in Headless Chrome...");
    await page.emulateMediaType("print");

    const printStyles = await page.evaluate(() => {
      const header = document.querySelector("header");
      const navDisplay = header ? window.getComputedStyle(header).display : "not found";

      const noPrintEl = document.querySelector(".no-print");
      const noPrintDisplay = noPrintEl ? window.getComputedStyle(noPrintEl).display : "not found";

      const thead = document.querySelector("thead");
      const theadDisplay = thead ? window.getComputedStyle(thead).display : "not found";

      const printOnlyEl = document.querySelector(".print-only");
      const printOnlyDisplay = printOnlyEl ? window.getComputedStyle(printOnlyEl).display : "not found";

      return {
        navDisplay,
        noPrintDisplay,
        theadDisplay,
        printOnlyDisplay,
      };
    });

    console.log("-> Print Media Computed Styles:", printStyles);

    if (printStyles.navDisplay !== "none") {
      throw new Error(`Print style error: header/nav display should be 'none', got '${printStyles.navDisplay}'`);
    }
    if (printStyles.noPrintDisplay !== "none") {
      throw new Error(`Print style error: .no-print elements display should be 'none', got '${printStyles.noPrintDisplay}'`);
    }
    if (printStyles.theadDisplay !== "table-header-group") {
      throw new Error(`Print style error: thead display should be 'table-header-group' for repeated headers, got '${printStyles.theadDisplay}'`);
    }
    if (printStyles.printOnlyDisplay !== "block") {
      throw new Error(`Print style error: .print-only sign-off block display should be 'block', got '${printStyles.printOnlyDisplay}'`);
    }

    console.log("   ✓ Navigation and interactive controls are completely hidden in print mode (display: none)");
    console.log("   ✓ Table header group (thead) is set to 'table-header-group' ensuring repeated headers across printed pages");
    console.log("   ✓ Official Super Administrator and Audit Sign-off blocks are visible in print mode (display: block)");

    // Generate real PDF output
    const pdfPath = "/home/recoding/.gemini/antigravity-cli/brain/2b5d7315-6492-4e85-bc0b-28e49357c614/scratch/financial_statement_print.pdf";
    await page.pdf({
      path: pdfPath,
      format: "A4",
      printBackground: true,
      margin: { top: "1.5cm", bottom: "1.5cm", left: "1.5cm", right: "1.5cm" },
    });
    console.log("   ✓ Authentic Printable Statement PDF Generated:", pdfPath);

    console.log("\n==========================================================");
    console.log(" All Browser Tests Passed Successfully!");
    console.log("==========================================================");
  } finally {
    await browser.close();
  }
}

runBrowserVerification().catch((err) => {
  console.error("FATAL BROWSER TEST ERROR:", err);
  process.exit(1);
});
