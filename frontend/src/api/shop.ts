import api from "./client";
import type {
  ShopInitResponse,
  ShopCategoriesResponse,
  ShopAllProductsResponse,
} from "../types/shop";

// Endpoints "compat" servis par internal/handler/api_compat.go.
// À terme, remplacer par des endpoints REST propres si on dégage la couche
// de compatibilité Haxe.

export function fetchShopInit(multiDistribId: number) {
  return api
    .get<ShopInitResponse>("/shop/init", { params: { multiDistrib: multiDistribId } })
    .then((r) => r.data);
}

export function fetchShopCategories(multiDistribId: number) {
  return api
    .get<ShopCategoriesResponse>("/shop/categories", { params: { multiDistrib: multiDistribId } })
    .then((r) => r.data);
}

export function fetchShopProducts(multiDistribId: number) {
  return api
    .get<ShopAllProductsResponse>("/shop/allProducts", { params: { multiDistrib: multiDistribId } })
    .then((r) => r.data);
}

// Payload aligné sur le handler Go ShopSubmit (/api/shop/submit/:multiDistribId).
// Une commande shop est toujours pour un (multiDistrib, catalog) donné : si le
// panier mélange plusieurs catalogues, l'appelant doit faire un appel par
// catalogue (cf. CartPanel).
export interface SubmitOrderPayload {
  catalogId: number;
  /** Optionnel : permet à un admin de groupe de commander pour un membre. */
  userId?: number;
  orders: Array<{ productId: number; qt: number }>;
}

export function submitOrder(multiDistribId: number, payload: SubmitOrderPayload) {
  return api
    .post<{ success: boolean; orders?: unknown[]; error?: string }>(
      `/shop/submit/${multiDistribId}`,
      payload,
    )
    .then((r) => r.data);
}
