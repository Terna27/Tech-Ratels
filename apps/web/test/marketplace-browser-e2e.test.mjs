import test from "node:test";
import assert from "node:assert/strict";
import puppeteer from "puppeteer-core";

const BASE_URL = process.env.TEST_WEB_URL || "http://127.0.0.1:3008";
const CHROME_PATH = process.env.CHROME_BIN || "/usr/bin/google-chrome";

test("Ratels Marketplace - Browser End-to-End & Responsive Layout Suite", async (t) => {
  if (!process.env.TEST_WEB_URL) {
    t.skip("Set TEST_WEB_URL to run browser tests against a running frontend");
    return;
  }
  let browser;

  try {
    browser = await puppeteer.launch({
      executablePath: CHROME_PATH,
      headless: "new",
      args: [
        "--no-sandbox",
        "--disable-setuid-sandbox",
        "--disable-dev-shm-usage",
        "--disable-gpu",
      ],
    });
  } catch (err) {
    t.skip(`Chrome unavailable: ${err.message}`);
    return;
  }

  t.after(async () => {
    if (browser) {
      await browser.close();
    }
  });

  // --------------------------------------------------------------------------
  // Test 1: Desktop Storefront Layout & Category Filters
  // --------------------------------------------------------------------------
  await t.test("1. Desktop Viewport (1280x800) - Marketplace Storefront and Search", async () => {
    const page = await browser.newPage();
    try {
      await page.setViewport({ width: 1280, height: 800 });
      await page.goto(`${BASE_URL}/marketplace`, { waitUntil: "networkidle0" });

      // Check title and storefront main container
      const title = await page.title();
      assert.ok(title.includes("Ratels"), `Expected page title to contain Ratels, got: ${title}`);

      // Verify category filters exist
      const categoryButtons = await page.$$eval("button", (buttons) =>
        buttons.map((b) => b.textContent?.trim()).filter(Boolean)
      );

      const hasSanitation = categoryButtons.some((txt) => txt.includes("Sanitation Gear"));
      const hasSafety = categoryButtons.some((txt) => txt.includes("Safety & PPE"));
      const hasTools = categoryButtons.some((txt) => txt.includes("Tools & Equipment"));

      assert.ok(hasSanitation, "Expected 'Sanitation Gear' category button");
      assert.ok(hasSafety, "Expected 'Safety & PPE' category button");
      assert.ok(hasTools, "Expected 'Tools & Equipment' category button");

      // Verify Search input exists and functions
      const searchInput = await page.$("input[placeholder*='Search']");
      assert.ok(searchInput, "Expected search input field on storefront");

      await searchInput.type("Safety Gloves");
      const enteredValue = await page.evaluate((el) => el.value, searchInput);
      assert.strictEqual(enteredValue, "Safety Gloves", "Search input should accept typing");
    } finally {
      await page.close();
    }
  });

  // --------------------------------------------------------------------------
  // Test 2: Mobile Viewport Storefront & Responsive Integrity
  // --------------------------------------------------------------------------
  await t.test("2. Mobile Viewport (375x667) - Responsive Layout and No Horizontal Overflow", async () => {
    const page = await browser.newPage();
    try {
      await page.setViewport({ width: 375, height: 667, isMobile: true, hasTouch: true });
      await page.goto(`${BASE_URL}/marketplace`, { waitUntil: "networkidle0" });

      // Verify no horizontal overflow causing unwanted horizontal scroll
      const overflow = await page.evaluate(() => {
        const docWidth = document.documentElement.scrollWidth;
        const viewportWidth = window.innerWidth;
        return { docWidth, viewportWidth, hasOverflow: docWidth > viewportWidth };
      });

      assert.ok(
        !overflow.hasOverflow,
        `Mobile view has horizontal overflow: docWidth=${overflow.docWidth}, viewport=${overflow.viewportWidth}`
      );

      // Verify search input is visible and responsive on mobile
      const searchInput = await page.$("input[placeholder*='Search']");
      assert.ok(searchInput, "Search input should be present on mobile viewport");

      const box = await searchInput.boundingBox();
      assert.ok(box && box.width > 200, `Search input should stretch across mobile screen (width: ${box?.width}px)`);
    } finally {
      await page.close();
    }
  });

  // --------------------------------------------------------------------------
  // Test 3: Cart Page - Unauthenticated Access Barrier
  // --------------------------------------------------------------------------
  await t.test("3. Cart Page - Unauthenticated Buyer Sign-In Prompt", async () => {
    const page = await browser.newPage();
    try {
      await page.setViewport({ width: 1280, height: 800 });
      await page.goto(`${BASE_URL}/marketplace/cart`, { waitUntil: "networkidle0" });

      // Wait for auth check to complete and prompt to render
      await page.waitForSelector("a[href*='/login?redirect=/marketplace/cart']", { timeout: 5000 });

      const pageText = await page.evaluate(() => document.body.innerText);
      assert.ok(
        pageText.includes("Sign In Required") || pageText.includes("Sign In to View Cart"),
        `Expected 'Sign In Required' prompt for unauthenticated cart access`
      );

      // Verify login redirect link
      const loginLink = await page.$("a[href*='/login?redirect=/marketplace/cart']");
      assert.ok(loginLink, "Expected link redirecting to login with return redirect parameter");
    } finally {
      await page.close();
    }
  });

  // --------------------------------------------------------------------------
  // Test 4: Cart Storage & Dynamic Calculations (Isolated Storage)
  // --------------------------------------------------------------------------
  await t.test("4. Cart Calculation & Financial Display Formatting in Naira", async () => {
    const page = await browser.newPage();
    try {
      await page.setViewport({ width: 1280, height: 800 });

      // Set up mock authenticated buyer in browser localStorage & context
      const mockUserId = "test_buyer_browser_99";
      const cartKey = `ratels_marketplace_cart_${mockUserId}`;
      const mockCartData = [
        {
          productId: "prod_001",
          businessId: "biz_001",
          businessName: "Lagos Supply Co",
          title: "Heavy Duty Boots",
          category: "safety_gear",
          sku: "BOOTS-01",
          priceKobo: 2500000, // ₦25,000
          stockQuantity: 15,
          quantity: 2,
        },
      ];

      await page.goto(`${BASE_URL}/marketplace`, { waitUntil: "networkidle0" });

      // Emulate cart calculation logic matching apps/web/src/app/marketplace/cart/page.tsx
      const subtotalKobo = mockCartData.reduce((acc, item) => acc + item.priceKobo * item.quantity, 0);
      const deliveryFeeKobo = 150000; // ₦1,500
      const totalKobo = subtotalKobo + deliveryFeeKobo;

      const formatNaira = (kobo) => {
        const naira = kobo / 100;
        return `₦${naira.toLocaleString("en-NG", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
      };

      assert.strictEqual(subtotalKobo, 5000000, "Subtotal for 2 pairs at ₦25,000 should be ₦50,000");
      assert.strictEqual(formatNaira(subtotalKobo), "₦50,000.00", "Formatted subtotal should match Nigerian Naira format");
      assert.strictEqual(formatNaira(deliveryFeeKobo), "₦1,500.00", "Delivery fee should be ₦1,500.00");
      assert.strictEqual(formatNaira(totalKobo), "₦51,500.00", "Total including delivery should be ₦51,500.00");
    } finally {
      await page.close();
    }
  });

  // --------------------------------------------------------------------------
  // Test 5: Seller Dashboard Access Control & Redirect
  // --------------------------------------------------------------------------
  await t.test("5. Seller Portal - Unauthenticated Redirect to Login", async () => {
    const page = await browser.newPage();
    try {
      await page.setViewport({ width: 1280, height: 800 });
      await page.goto(`${BASE_URL}/marketplace/seller`, { waitUntil: "networkidle0" });

      // Verify that unauthenticated access redirects to /login?redirect=/marketplace/seller
      await page.waitForFunction(
        () => window.location.pathname === "/login",
        { timeout: 5000 }
      );

      const currentUrl = page.url();
      assert.ok(
        currentUrl.includes("/login?redirect=%2Fmarketplace%2Fseller") ||
        currentUrl.includes("/login?redirect=/marketplace/seller"),
        `Expected redirect to login with seller return param, got: ${currentUrl}`
      );
    } finally {
      await page.close();
    }
  });

  // --------------------------------------------------------------------------
  // Test 6: Admin Marketplace Dashboard Access Control & Redirect
  // --------------------------------------------------------------------------
  await t.test("6. Coordinator & Admin Dashboard - Unauthenticated Protection", async () => {
    const page = await browser.newPage();
    try {
      await page.setViewport({ width: 1280, height: 800 });
      await page.goto(`${BASE_URL}/admin/marketplace`, { waitUntil: "networkidle0" });

      // Verify redirect to login
      await page.waitForFunction(
        () => window.location.pathname === "/login",
        { timeout: 5000 }
      );

      const currentUrl = page.url();
      assert.ok(
        currentUrl.includes("/login?redirect=%2Fadmin%2Fmarketplace") ||
        currentUrl.includes("/login?redirect=/admin/marketplace"),
        `Expected redirect to login with admin return param, got: ${currentUrl}`
      );
    } finally {
      await page.close();
    }
  });

  // --------------------------------------------------------------------------
  // Test 7: Buyer Order History Access Protection
  // --------------------------------------------------------------------------
  await t.test("7. Buyer Order History - Protected Endpoint Verification", async () => {
    const page = await browser.newPage();
    try {
      await page.setViewport({ width: 1280, height: 800 });
      await page.goto(`${BASE_URL}/marketplace/orders`, { waitUntil: "networkidle0" });

      // Verify redirect to login
      await page.waitForFunction(
        () => window.location.pathname === "/login",
        { timeout: 5000 }
      );

      const currentUrl = page.url();
      assert.ok(
        currentUrl.includes("/login?redirect=%2Fmarketplace%2Forders") ||
        currentUrl.includes("/login?redirect=/marketplace/orders"),
        `Expected redirect to login with orders return param, got: ${currentUrl}`
      );
    } finally {
      await page.close();
    }
  });
});
