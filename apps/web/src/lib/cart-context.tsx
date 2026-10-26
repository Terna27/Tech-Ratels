"use client";

import React, { createContext, useContext, useEffect, useState, useRef } from "react";
import { CartItem } from "./types";
import { useAuth } from "./auth-context";

interface CartContextType {
  items: CartItem[];
  addToCart: (item: CartItem) => void;
  updateQuantity: (productId: string, quantity: number) => void;
  removeFromCart: (productId: string) => void;
  clearCart: () => void;
  totalItems: number;
  subtotalKobo: number;
  isLoaded: boolean;
}

const CartContext = createContext<CartContextType | undefined>(undefined);

const LEGACY_CART_STORAGE_KEY = "ratels_marketplace_cart_v1";

export function getCartStorageKey(userId: string): string {
  return `ratels_marketplace_cart_${userId}`;
}

export function CartProvider({ children }: { children: React.ReactNode }) {
  const { user, isLoading: isAuthLoading } = useAuth();
  const [items, setItems] = useState<CartItem[]>([]);
  const [isLoaded, setIsLoaded] = useState(false);

  // Tracks the user ID whose cart is currently loaded in state
  const activeUserIdRef = useRef<string | null>(null);

  // 1. Unconditionally remove the legacy shared cart key without importing into any account
  useEffect(() => {
    try {
      if (typeof window !== "undefined") {
        localStorage.removeItem(LEGACY_CART_STORAGE_KEY);
      }
    } catch {
      // Ignore storage access errors
    }
  }, []);

  // 2. Synchronize active cart state with authentication state
  useEffect(() => {
    // Keep cart contents hidden while authentication is loading
    if (isAuthLoading) {
      setItems([]);
      setIsLoaded(false);
      activeUserIdRef.current = null;
      return;
    }

    const currentUserId = user?.id || null;

    // Unauthenticated: clear active items from memory, hide from view
    if (!currentUserId) {
      setItems([]);
      setIsLoaded(true);
      activeUserIdRef.current = null;
      return;
    }

    // Authenticated user: restore cart only for this authenticated user ID
    if (activeUserIdRef.current !== currentUserId) {
      activeUserIdRef.current = currentUserId;
      try {
        const storageKey = getCartStorageKey(currentUserId);
        const saved = typeof window !== "undefined" ? localStorage.getItem(storageKey) : null;
        if (saved) {
          const parsed = JSON.parse(saved);
          if (Array.isArray(parsed)) {
            setItems(parsed);
          } else {
            setItems([]);
          }
        } else {
          setItems([]);
        }
      } catch {
        setItems([]);
      } finally {
        setIsLoaded(true);
      }
    }
  }, [user?.id, isAuthLoading]);

  // 3. Save to localStorage strictly for the active authenticated user
  useEffect(() => {
    // Prevent stale cart state from saving under another user's key or unauthenticated storage
    if (!isLoaded || isAuthLoading || !user?.id) {
      return;
    }

    if (activeUserIdRef.current !== user.id) {
      return;
    }

    try {
      if (typeof window !== "undefined") {
        const storageKey = getCartStorageKey(user.id);
        localStorage.setItem(storageKey, JSON.stringify(items));
      }
    } catch {
      // Ignore storage quota errors
    }
  }, [items, isLoaded, isAuthLoading, user?.id]);

  const addToCart = (newItem: CartItem) => {
    // Require authentication before adding items to a persistent cart
    if (!user?.id) {
      if (typeof window !== "undefined") {
        const redirectUrl = encodeURIComponent(window.location.pathname + window.location.search);
        window.location.href = `/login?redirect=${redirectUrl}`;
      }
      return;
    }

    setItems((prev) => {
      const idx = prev.findIndex((i) => i.productId === newItem.productId);
      if (idx > -1) {
        const updated = [...prev];
        updated[idx] = {
          ...updated[idx],
          quantity: updated[idx].quantity + newItem.quantity,
        };
        return updated;
      }
      return [...prev, newItem];
    });
  };

  const updateQuantity = (productId: string, quantity: number) => {
    if (!user?.id) return;
    setItems((prev) => {
      if (quantity <= 0) {
        return prev.filter((i) => i.productId !== productId);
      }
      return prev.map((i) =>
        i.productId === productId ? { ...i, quantity } : i
      );
    });
  };

  const removeFromCart = (productId: string) => {
    if (!user?.id) return;
    setItems((prev) => prev.filter((i) => i.productId !== productId));
  };

  const clearCart = () => {
    setItems([]);
    if (user?.id && typeof window !== "undefined") {
      try {
        localStorage.removeItem(getCartStorageKey(user.id));
      } catch {
        // Ignore storage errors
      }
    }
  };

  // Badge count and subtotal: only computed for the authenticated user
  const totalItems = user ? items.reduce((sum, itm) => sum + itm.quantity, 0) : 0;
  const subtotalKobo = user ? items.reduce((sum, itm) => sum + itm.priceKobo * itm.quantity, 0) : 0;

  return (
    <CartContext.Provider
      value={{
        items: user ? items : [],
        addToCart,
        updateQuantity,
        removeFromCart,
        clearCart,
        totalItems,
        subtotalKobo,
        isLoaded,
      }}
    >
      {children}
    </CartContext.Provider>
  );
}

export function useCart() {
  const ctx = useContext(CartContext);
  if (!ctx) {
    throw new Error("useCart must be used within a CartProvider");
  }
  return ctx;
}
