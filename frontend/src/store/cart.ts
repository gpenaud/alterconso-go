import { create } from "zustand";
import type { ProductInfo } from "../types/shop";

// Pas de persistance localStorage : un panier représente une session
// d'édition transitoire. Quitter le shop sans valider doit perdre le panier
// (cf. ShopPage : cleanup d'unmount qui appelle clear()). Au retour dans le
// shop, le pre-fill réinitialise depuis les commandes serveur.

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
  // Frais en % appliqués au total. Hérité du Catalog.PercentageFees au
  // moment de l'ajout — UserOrder.FeesRate côté serveur est snapshotté de
  // la même manière, donc le calcul du panier reflète exactement ce que
  // /home (UserOrder.TotalPrice) affichera après submit.
  feesRate?: number | null;
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
  /** Sous-total HT/HF — somme price×quantity sans frais. */
  subtotal: () => number;
  /** Total des frais (% Catalog.PercentageFees appliqué ligne par ligne). */
  feesTotal: () => number;
  /** Total à payer = subtotal + feesTotal. C'est ce qu'affichera /home. */
  total: () => number;
  quantityOf: (productId: number) => number;
}

export const useCartStore = create<CartState>()((set, get) => ({
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
        feesRate: product.catalogTax ?? null,
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
  subtotal: () =>
    get().items.reduce((s, it) => s + it.price * it.quantity, 0),
  feesTotal: () =>
    get().items.reduce(
      (s, it) => s + it.price * it.quantity * ((it.feesRate ?? 0) / 100),
      0,
    ),
  total: () =>
    get().items.reduce(
      (s, it) => s + it.price * it.quantity * (1 + (it.feesRate ?? 0) / 100),
      0,
    ),
  quantityOf: (productId) => {
    const it = get().items.find((it) => it.productId === productId);
    return it ? it.quantity : 0;
  },
}));
