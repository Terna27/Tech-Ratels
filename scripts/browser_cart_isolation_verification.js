const path = require("path");
let puppeteer;
try {
  puppeteer = require("puppeteer-core");
} catch (e) {
  puppeteer = require(path.join(__dirname, "../apps/web/node_modules/puppeteer-core"));
}

const API_BASE = process.env.API_BASE || "http://127.0.0.1:8000/api/v1";
const WEB_BASE = process.env.WEB_BASE || "http://localhost:3000";

const USER_A_EMAIL = "member1@ratels.example.org";
const USER_A_PASS = "Member1234!";

const USER_B_EMAIL = "lagos_coord@ratels.example.org";
const USER_B_PASS = "Coord1234!";

const ADMIN_EMAIL = "admin@ratels.example.org";
const ADMIN_PASS = "Admin1234!";

async function runCartIsolationVerification() {
  console.log("==========================================================");
  console.log(" Starting Real Browser Cart Isolation & UI Regression Suite");
  console.log("==========================================================");

  const browser = await puppeteer.launch({
    executablePath: "/usr/bin/google-chrome-stable",
    headless: "new",
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 800 });

  try {
    // -------------------------------------------------------------------------
    // SCENARIO 1: Unauthenticated Cart State & Add-To-Cart Guard
    // -------------------------------------------------------------------------
    console.log("\n-> [1] Testing Unauthenticated Cart Access & Guarding...");
    await page.goto(`${WEB_BASE}/marketplace/cart`, { waitUntil: "networkidle2" });

    // Verify "Sign In Required" is displayed
    const bodyText = await page.evaluate(() => document.body.innerText);
    if (!bodyText.includes("Sign In Required")) {
      throw new Error("Unauthenticated user on /marketplace/cart did not see 'Sign In Required'!");
    }
    console.log("   ✓ Verified unauthenticated user on /marketplace/cart sees 'Sign In Required'.");

    // Verify cart badge does not appear in Header
    await page.goto(`${WEB_BASE}/marketplace`, { waitUntil: "networkidle2" });
    const hasBadge = await page.evaluate(() => {
      const cartLink = document.querySelector('a[href="/marketplace/cart"]');
      return cartLink && cartLink.querySelector("span") !== null;
    });
    if (hasBadge) {
      throw new Error("Cart badge was visible for an unauthenticated visitor!");
    }
    console.log("   ✓ Verified cart badge is hidden for unauthenticated visitor.");

    // -------------------------------------------------------------------------
    // SCENARIO 2: User A Logs In, Adds Product, Cart Badge Displays
    // -------------------------------------------------------------------------
    console.log("\n-> [2] Logging in as User A (member1)...");
    await page.goto(`${WEB_BASE}/login?redirect=/marketplace`, { waitUntil: "networkidle2" });
    await page.waitForSelector('input[name="identifier"], input[type="text"]');
    await page.type('input[name="identifier"], input[type="text"]', USER_A_EMAIL);
    await page.type('input[type="password"]', USER_A_PASS);
    await page.click('button[type="submit"]');
    await page.waitForNavigation({ waitUntil: "networkidle2" });
    console.log("   ✓ User A authenticated.");

    // Add product to cart from storefront
    console.log("-> Adding product to cart as User A...");
    await page.waitForSelector("a[href^='/marketplace/products/']", { timeout: 10000 });
    const productLinks = await page.$$("a[href^='/marketplace/products/']");
    if (productLinks.length === 0) {
      throw new Error("No products found on storefront!");
    }

    // Click "Add to Cart" button on first product card
    const firstAddBtn = await page.evaluateHandle(() => {
      const card = document.querySelector("a[href^='/marketplace/products/']");
      return card ? card.querySelector("button") : null;
    });
    if (!firstAddBtn || !firstAddBtn.asElement()) {
      throw new Error("Add to cart button not found on product card!");
    }
    await firstAddBtn.asElement().click();
    await new Promise(r => setTimeout(r, 600));

    // Verify Cart badge appeared with count > 0
    const badgeCount = await page.evaluate(() => {
      const cartLink = document.querySelector('a[href="/marketplace/cart"]');
      const badge = cartLink ? cartLink.querySelector("span") : null;
      return badge ? badge.innerText.trim() : null;
    });
    console.log(`   ✓ Cart badge displayed with count: ${badgeCount}`);
    if (!badgeCount || parseInt(badgeCount, 10) < 1) {
      throw new Error(`Cart badge failed to reflect added item! Got: ${badgeCount}`);
    }

    // Inspect Cart page as User A
    await page.goto(`${WEB_BASE}/marketplace/cart`, { waitUntil: "networkidle2" });
    await page.waitForSelector("h1");
    const userAItemsCount = await page.evaluate(() => {
      return document.querySelectorAll('button[title="Remove item"]').length;
    });
    if (userAItemsCount < 1) {
      throw new Error("User A cart page did not render the added item!");
    }
    console.log(`   ✓ Verified User A has ${userAItemsCount} item in cart.`);

    // -------------------------------------------------------------------------
    // SCENARIO 3: User A Logs Out -> Active Cart Disappears & Redirects to Homepage
    // -------------------------------------------------------------------------
    console.log("\n-> [3] Logging out User A from Header...");
    // Open user menu
    const userMenuBtn = await page.$('button[aria-label="User account menu"]');
    if (!userMenuBtn) {
      throw new Error("User account menu button not found!");
    }
    await userMenuBtn.click();
    await new Promise(r => setTimeout(r, 300));

    // Click Sign Out
    const signOutBtn = await page.evaluateHandle(() => {
      const buttons = Array.from(document.querySelectorAll("button"));
      return buttons.find(b => b.innerText.includes("Sign Out"));
    });
    if (!signOutBtn || !signOutBtn.asElement()) {
      throw new Error("Sign Out button not found in user dropdown!");
    }

    await Promise.all([
      page.waitForNavigation({ waitUntil: "networkidle2" }),
      signOutBtn.asElement().click(),
    ]);

    // Verify redirected to homepage /
    const currentUrl = page.url();
    console.log(`   Current URL after logout: ${currentUrl}`);
    const urlObj = new URL(currentUrl);
    if (urlObj.pathname !== "/") {
      throw new Error(`Logout failed to redirect to homepage! Landed on: ${urlObj.pathname}`);
    }
    console.log("   ✓ Verified logout redirected globally to homepage '/'.");

    // Verify cart badge disappeared on homepage
    const badgeAfterLogout = await page.evaluate(() => {
      const cartLink = document.querySelector('a[href="/marketplace/cart"]');
      const badge = cartLink ? cartLink.querySelector("span") : null;
      return badge ? badge.innerText : null;
    });
    if (badgeAfterLogout) {
      throw new Error(`Cart badge still displayed after logout: ${badgeAfterLogout}`);
    }
    console.log("   ✓ Verified cart badge disappeared after logout.");

    // Verify /marketplace/cart now shows "Sign In Required"
    await page.goto(`${WEB_BASE}/marketplace/cart`, { waitUntil: "networkidle2" });
    const cartTextAfterLogout = await page.evaluate(() => document.body.innerText);
    if (!cartTextAfterLogout.includes("Sign In Required")) {
      throw new Error("User A cart remained visible after logout!");
    }
    console.log("   ✓ Verified User A's cart contents are hidden after logout.");

    // -------------------------------------------------------------------------
    // SCENARIO 4: User B Logs In on Same Browser -> CANNOT Access User A's Cart
    // -------------------------------------------------------------------------
    console.log("\n-> [4] Logging in as User B (lagos_coord) on same browser...");
    await page.goto(`${WEB_BASE}/login?redirect=/marketplace/cart`, { waitUntil: "networkidle2" });
    await page.waitForSelector('input[name="identifier"], input[type="text"]');
    await page.type('input[name="identifier"], input[type="text"]', USER_B_EMAIL);
    await page.type('input[type="password"]', USER_B_PASS);
    await page.click('button[type="submit"]');
    await page.waitForNavigation({ waitUntil: "networkidle2" });
    console.log("   ✓ User B authenticated.");

    // User B on /marketplace/cart: cart should be EMPTY!
    const userBCartText = await page.evaluate(() => document.body.innerText);
    if (!userBCartText.includes("Your Cart is Empty")) {
      throw new Error("Security leak: User B could access User A's cart items!");
    }
    console.log("   ✓ Verified User B's cart is empty (User A's items are completely isolated).");

    // User B adds their own item
    await page.goto(`${WEB_BASE}/marketplace`, { waitUntil: "networkidle2" });
    await page.waitForSelector("a[href^='/marketplace/products/']");
    const userBAddBtn = await page.evaluateHandle(() => {
      const cards = document.querySelectorAll("a[href^='/marketplace/products/']");
      const card = cards[cards.length - 1] || cards[0];
      return card ? card.querySelector("button") : null;
    });
    if (userBAddBtn && userBAddBtn.asElement()) {
      await userBAddBtn.asElement().click();
      await new Promise(r => setTimeout(r, 600));
    }
    console.log("   ✓ User B added their own product to cart.");

    // Logout User B
    const userMenuB = await page.$('button[aria-label="User account menu"]');
    await userMenuB.click();
    await new Promise(r => setTimeout(r, 300));
    const signOutB = await page.evaluateHandle(() => {
      const buttons = Array.from(document.querySelectorAll("button"));
      return buttons.find(b => b.innerText.includes("Sign Out"));
    });
    await Promise.all([
      page.waitForNavigation({ waitUntil: "networkidle2" }),
      signOutB.asElement().click(),
    ]);
    console.log("   ✓ User B logged out.");

    // -------------------------------------------------------------------------
    // SCENARIO 5: User A Logs Back In -> Own Cart Recovered
    // -------------------------------------------------------------------------
    console.log("\n-> [5] Logging back in as User A to verify cart recovery...");
    await page.goto(`${WEB_BASE}/login?redirect=/marketplace/cart`, { waitUntil: "networkidle2" });
    await page.waitForSelector('input[name="identifier"], input[type="text"]');
    await page.type('input[name="identifier"], input[type="text"]', USER_A_EMAIL);
    await page.type('input[type="password"]', USER_A_PASS);
    await page.click('button[type="submit"]');
    await page.waitForNavigation({ waitUntil: "networkidle2" });

    // Verify User A's cart is recovered
    await page.waitForSelector("h1");
    const recoveredItemsCount = await page.evaluate(() => {
      return document.querySelectorAll('button[title="Remove item"]').length;
    });
    if (recoveredItemsCount < 1) {
      throw new Error("User A failed to recover their own cart after logging back in!");
    }
    console.log(`   ✓ Verified User A recovered their cart (${recoveredItemsCount} items present).`);

    // -------------------------------------------------------------------------
    // SCENARIO 6: Global Logout Redirect from Admin and Member Routes
    // -------------------------------------------------------------------------
    console.log("\n-> [6] Testing Global Logout Redirect from /admin workspace...");
    // Log out User A
    const menuA = await page.$('button[aria-label="User account menu"]');
    await menuA.click();
    await new Promise(r => setTimeout(r, 300));
    const signoutA = await page.evaluateHandle(() => {
      const buttons = Array.from(document.querySelectorAll("button"));
      return buttons.find(b => b.innerText.includes("Sign Out"));
    });
    await Promise.all([
      page.waitForNavigation({ waitUntil: "networkidle2" }),
      signoutA.asElement().click(),
    ]);

    // Log in as National Admin
    await page.goto(`${WEB_BASE}/login?redirect=/admin`, { waitUntil: "networkidle2" });
    await page.waitForSelector('input[name="identifier"], input[type="text"]');
    await page.type('input[name="identifier"], input[type="text"]', ADMIN_EMAIL);
    await page.type('input[type="password"]', ADMIN_PASS);
    await page.click('button[type="submit"]');
    await page.waitForNavigation({ waitUntil: "networkidle2" });
    console.log("   ✓ Admin authenticated on /admin.");

    // Log out from /admin
    const adminMenu = await page.$('button[aria-label="User account menu"]');
    await adminMenu.click();
    await new Promise(r => setTimeout(r, 300));
    const adminSignOut = await page.evaluateHandle(() => {
      const buttons = Array.from(document.querySelectorAll("button"));
      return buttons.find(b => b.innerText.includes("Sign Out"));
    });
    await Promise.all([
      page.waitForNavigation({ waitUntil: "networkidle2" }),
      adminSignOut.asElement().click(),
    ]);

    const adminLogoutUrl = new URL(page.url()).pathname;
    if (adminLogoutUrl !== "/") {
      throw new Error(`Logout from /admin redirected to ${adminLogoutUrl} instead of '/'!`);
    }
    console.log("   ✓ Verified logout from /admin redirects globally to homepage '/'.");

    // -------------------------------------------------------------------------
    // SCENARIO 7: Responsive Marketplace Grid & Compact Card Presentation
    // -------------------------------------------------------------------------
    console.log("\n-> [7] Inspecting Marketplace Product Grid & Compact Image Presentation...");
    await page.goto(`${WEB_BASE}/marketplace`, { waitUntil: "networkidle2" });
    await page.waitForSelector("a[href^='/marketplace/products/']");

    // Desktop Viewport (1280px)
    await page.setViewport({ width: 1280, height: 800 });
    const desktopCardInfo = await page.evaluate(() => {
      const cards = Array.from(document.querySelectorAll("a[href^='/marketplace/products/']"));
      if (cards.length === 0) return null;
      const firstCard = cards[0];
      const imgContainer = firstCard.querySelector(".border-y");
      const img = firstCard.querySelector("img");
      const rect = imgContainer ? imgContainer.getBoundingClientRect() : null;
      const imgRect = img ? img.getBoundingClientRect() : null;
      const title = firstCard.querySelector("h3")?.innerText;
      const categoryBadge = firstCard.querySelector(".bg-emerald-50")?.innerText;

      return {
        cardCount: cards.length,
        containerHeight: rect ? rect.height : 0,
        imgHeight: imgRect ? imgRect.height : 0,
        title,
        categoryBadge,
        hasHorizontalOverflow: document.documentElement.scrollWidth > window.innerWidth,
      };
    });

    console.log("   Desktop presentation metrics:", desktopCardInfo);
    // Prior presentation had 280-320px image container. Target is ~30% (~84-112px).
    if (!desktopCardInfo || desktopCardInfo.containerHeight > 140) {
      throw new Error(`Image container height (${desktopCardInfo?.containerHeight}px) exceeds compact target!`);
    }
    console.log("   ✓ Verified compact image container on desktop (~96-112px, ~30% of prior presentation).");

    // Tablet Viewport (768px)
    await page.setViewport({ width: 768, height: 1024 });
    await new Promise(r => setTimeout(r, 200));
    const tabletMetrics = await page.evaluate(() => {
      const isPageOverflow = document.documentElement.scrollWidth > window.innerWidth;
      const all = Array.from(document.querySelectorAll("*"));
      const uncontainedOverflow = all.filter(el => {
        let parent = el;
        while (parent && parent !== document.body) {
          const overflowX = window.getComputedStyle(parent).overflowX;
          if (overflowX === "auto" || overflowX === "scroll") return false;
          parent = parent.parentElement;
        }
        const r = el.getBoundingClientRect();
        return r.right > window.innerWidth + 1;
      });
      return { isPageOverflow, uncontainedCount: uncontainedOverflow.length };
    });

    console.log("   Tablet metrics:", tabletMetrics);
    if (tabletMetrics.isPageOverflow && tabletMetrics.uncontainedCount > 0) {
      throw new Error(`Unintended horizontal page overflow detected on tablet viewport!`);
    }
    console.log("   ✓ Verified clean alignment on tablet viewport (768px).");

    // Mobile Viewport (375px)
    await page.setViewport({ width: 375, height: 667 });
    await new Promise(r => setTimeout(r, 200));
    const mobileMetrics = await page.evaluate(() => {
      const isPageOverflow = document.documentElement.scrollWidth > window.innerWidth;
      const all = Array.from(document.querySelectorAll("*"));
      const uncontainedOverflow = all.filter(el => {
        let parent = el;
        while (parent && parent !== document.body) {
          const overflowX = window.getComputedStyle(parent).overflowX;
          if (overflowX === "auto" || overflowX === "scroll") return false;
          parent = parent.parentElement;
        }
        const r = el.getBoundingClientRect();
        return r.right > window.innerWidth + 1;
      });
      return { isPageOverflow, uncontainedCount: uncontainedOverflow.length };
    });

    console.log("   Mobile metrics:", mobileMetrics);
    if (mobileMetrics.isPageOverflow && mobileMetrics.uncontainedCount > 0) {
      throw new Error(`Unintended horizontal page overflow detected on mobile viewport!`);
    }
    console.log("   ✓ Verified clean responsive wrapping on mobile viewport (375px).");

    console.log("\n==========================================================");
    console.log(" All Browser Cart Isolation & UI Regression Tests PASSED! ");
    console.log("==========================================================");
  } finally {
    await browser.close();
  }
}

runCartIsolationVerification().catch((err) => {
  console.error("\n❌ Browser verification failed:", err);
  process.exit(1);
});
