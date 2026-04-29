import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { ProductInfo } from "../types/shop";

export interface CartItem {
  productId: number;
  quantity: number;
  // Snapshot pour l'affichage indépendant des refresh produits.
  name: string;
  price: number;
  image?: string | null;
  vendorId?: number;
  catalogId?: number;
  // Champs unitaires conservés pour afficher la qté totale (qt × quantity).
  qt?: number | null;
  unitType?: import("../types/shop").Unit | number | null;
}

interface CartState {
  multiDistribId: number | null;
  items: CartItem[];

  setMultiDistrib: (id: number) => void;
  add: (product: ProductInfo, qty?: number) => void;
  setQuantity: (productId: number, qty: number) => void;
  remove: (productId: number) => void;
  clear: () => void;
  replace: (items: CartItem[]) => void;

  count: () => number;
  total: () => number;
  quantityOf: (productId: number) => number;
}

export const useCartStore = create<CartState>()(
  persist(
    (set, get) => ({
      multiDistribId: null,
      items: [],

      setMultiDistrib: (id) => {
        // Si le shop change de MultiDistrib, on remet à zéro le panier — un
        // panier appartient à une distribution donnée.
        const prev = get().multiDistribId;
        if (prev !== id) {
          set({ multiDistribId: id, items: [] });
        }
      },

      add: (product, qty = 1) => {
        const items = get().items.slice();
        const idx = items.findIndex((it) => it.productId === product.id);
        if (idx >= 0) {
          items[idx] = { ...items[idx], quantity: items[idx].quantity + qty };
        } else {
          items.push({
            productId: product.id,
            quantity: qty,
            name: product.name,
            price: product.price,
            image: product.image ?? null,
            vendorId: product.vendorId,
            catalogId: product.catalogId,
            qt: product.qt ?? null,
            unitType: product.unitType ?? null,
          });
        }
        set({ items });
      },

      setQuantity: (productId, qty) => {
        if (qty <= 0) {
          set({ items: get().items.filter((it) => it.productId !== productId) });
          return;
        }
        set({
          items: get().items.map((it) =>
            it.productId === productId ? { ...it, quantity: qty } : it,
          ),
        });
      },

      remove: (productId) =>
        set({ items: get().items.filter((it) => it.productId !== productId) }),

      clear: () => set({ items: [] }),

      replace: (items) => set({ items }),

      count: () => get().items.reduce((n, it) => n + it.quantity, 0),
      total: () =>
        get().items.reduce((s, it) => s + it.price * it.quantity, 0),
      quantityOf: (productId) => {
        const it = get().items.find((it) => it.productId === productId);
        return it ? it.quantity : 0;
      },
    }),
    {
      name: "alterconso-cart",
      partialize: (s) => ({ multiDistribId: s.multiDistribId, items: s.items }),
    },
  ),
);
