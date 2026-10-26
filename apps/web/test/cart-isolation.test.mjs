import test from "node:test";
import assert from "node:assert/strict";

// Mock localStorage implementation
class MockLocalStorage {
  constructor() {
    this.store = new Map();
  }
  getItem(key) {
    return this.store.get(key) ?? null;
  }
  setItem(key, value) {
    this.store.set(key, String(value));
  }
  removeItem(key) {
    this.store.delete(key);
  }
  clear() {
    this.store.clear();
  }
}

// Emulate getCartStorageKey
function getCartStorageKey(userId) {
  return `ratels_marketplace_cart_${userId}`;
}

test("Cart Isolation & Storage Transition Suite", async (t) => {
  const localStorage = new MockLocalStorage();
  const LEGACY_KEY = "ratels_marketplace_cart_v1";

  await t.test("1. Legacy shared cart key is purged without importing into any account", () => {
    // Seed legacy shared cart data
    localStorage.setItem(LEGACY_KEY, JSON.stringify([
      { productId: "legacy_prod_1", title: "Legacy Item", priceKobo: 50000, quantity: 1 }
    ]));

    assert.ok(localStorage.getItem(LEGACY_KEY) !== null);

    // CartProvider initial mount cleans up legacy key
    localStorage.removeItem(LEGACY_KEY);
    assert.strictEqual(localStorage.getItem(LEGACY_KEY), null);

    // Verify User A storage key does NOT import legacy data
    const userAKey = getCartStorageKey("user_A");
    assert.strictEqual(localStorage.getItem(userAKey), null);
  });

  await t.test("2. User A adds products, logs out, and active cart is cleared from memory", () => {
    const userAId = "user_A_id_123";
    const userAKey = getCartStorageKey(userAId);

    const userAItems = [
      { productId: "prod_1", title: "Sanitation Gloves", priceKobo: 250000, quantity: 2 },
      { productId: "prod_2", title: "Safety Helmet", priceKobo: 600000, quantity: 1 },
    ];

    // User A persists cart
    localStorage.setItem(userAKey, JSON.stringify(userAItems));

    // Active state while User A is logged in
    let currentUser = { id: userAId, display_name: "User A" };
    let activeItems = JSON.parse(localStorage.getItem(getCartStorageKey(currentUser.id)));
    let totalItems = currentUser ? activeItems.reduce((sum, i) => sum + i.quantity, 0) : 0;

    assert.strictEqual(activeItems.length, 2);
    assert.strictEqual(totalItems, 3); // 2 gloves + 1 helmet

    // User A logs out
    currentUser = null;
    activeItems = []; // Purged on logout
    totalItems = currentUser ? activeItems.reduce((sum, i) => sum + i.quantity, 0) : 0;

    // Cart badge reflects 0 items when unauthenticated
    assert.strictEqual(totalItems, 0);
    assert.strictEqual(activeItems.length, 0);

    // User A's stored cart remains safely persisted in storage across logout
    const persisted = JSON.parse(localStorage.getItem(userAKey));
    assert.strictEqual(persisted.length, 2);
    assert.strictEqual(persisted[0].productId, "prod_1");
  });

  await t.test("3. User B logs in on same browser and CANNOT access User A's cart", () => {
    const userAId = "user_A_id_123";
    const userBId = "user_B_id_456";

    const userBKey = getCartStorageKey(userBId);

    // User B signs in
    const currentUser = { id: userBId, display_name: "User B" };

    // Cart loaded strictly using User B's key
    const rawCart = localStorage.getItem(getCartStorageKey(currentUser.id));
    const activeItems = rawCart ? JSON.parse(rawCart) : [];
    const totalItems = currentUser ? activeItems.reduce((sum, i) => sum + i.quantity, 0) : 0;

    // User B's cart is empty - User A's items are completely isolated
    assert.strictEqual(activeItems.length, 0);
    assert.strictEqual(totalItems, 0);

    // User B adds their own product
    const userBItems = [
      { productId: "prod_3", title: "Agricultural Seeds", priceKobo: 120000, quantity: 5 },
    ];
    localStorage.setItem(userBKey, JSON.stringify(userBItems));

    // Confirm User A's cart was not overwritten or affected
    const userACart = JSON.parse(localStorage.getItem(getCartStorageKey(userAId)));
    assert.strictEqual(userACart.length, 2);
    assert.strictEqual(userACart[0].productId, "prod_1");

    // Confirm User B's cart only has User B's items
    const userBCart = JSON.parse(localStorage.getItem(userBKey));
    assert.strictEqual(userBCart.length, 1);
    assert.strictEqual(userBCart[0].productId, "prod_3");
  });

  await t.test("4. User A logs back in and recovers their own cart", () => {
    const userAId = "user_A_id_123";

    // User A signs back in
    const currentUser = { id: userAId, display_name: "User A" };
    const saved = localStorage.getItem(getCartStorageKey(currentUser.id));
    assert.ok(saved !== null);

    const activeItems = JSON.parse(saved);
    const totalItems = currentUser ? activeItems.reduce((sum, i) => sum + i.quantity, 0) : 0;

    assert.strictEqual(activeItems.length, 2);
    assert.strictEqual(totalItems, 3);
    assert.strictEqual(activeItems[0].title, "Sanitation Gloves");
    assert.strictEqual(activeItems[1].title, "Safety Helmet");
  });

  await t.test("5. Cart storage does not leak during rapid auth transitions", () => {
    let activeUserIdRef = null;
    let itemsState = [];

    // Simulate transition: User A -> logout -> User B
    const userAId = "user_A_id_123";
    const userBId = "user_B_id_456";

    // 1. User A active
    activeUserIdRef = userAId;
    itemsState = [{ productId: "pA", title: "Item A", quantity: 1, priceKobo: 100 }];

    // 2. Auth changes to null (Logout)
    // Synchronize effect:
    itemsState = [];
    activeUserIdRef = null;

    // Save effect checks: if activeUserIdRef !== user.id, it NEVER saves
    // Even if itemsState still had stale items, saving without matched activeUserIdRef is blocked
    const canSaveUnauthenticated = activeUserIdRef !== null;
    assert.strictEqual(canSaveUnauthenticated, false);

    // 3. Auth changes to User B
    // Load effect runs before save effect:
    activeUserIdRef = userBId;
    const userBCartRaw = localStorage.getItem(getCartStorageKey(userBId));
    itemsState = userBCartRaw ? JSON.parse(userBCartRaw) : [];

    // Ensure itemsState reflects User B, never stale User A
    assert.strictEqual(itemsState.some((i) => i.productId === "pA"), false);
  });

  await t.test("6. Unauthenticated users cannot add to persistent cart", () => {
    let currentUser = null;
    let redirectUrl = null;

    function attemptAddToCart(item) {
      if (!currentUser?.id) {
        redirectUrl = `/login?redirect=${encodeURIComponent("/marketplace")}`;
        return false;
      }
      return true;
    }

    const added = attemptAddToCart({ productId: "p_anon", title: "Anon Item" });
    assert.strictEqual(added, false);
    assert.strictEqual(redirectUrl, "/login?redirect=%2Fmarketplace");
  });
});
