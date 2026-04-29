import { useQuery } from "@tanstack/react-query";
import {
  fetchShopInit,
  fetchShopCategories,
  fetchShopProducts,
  fetchShopMe,
  fetchExistingOrders,
} from "../api/shop";
import { buildCatalog } from "../utils/catalog";
import type { CategoryInfo, FilteredProductCatalog } from "../types/shop";

/** Commandes déjà passées par l'utilisateur pour cette MultiDistrib —
 *  utilisé par ShopPage pour pré-remplir le panier en cas de modification. */
export function useExistingOrders(userId: number | undefined, multiDistribId: number) {
  const enabled = !!userId && Number.isInteger(multiDistribId) && multiDistribId > 0;
  return useQuery({
    queryKey: ["shop", "existingOrders", userId, multiDistribId],
    queryFn: () => fetchExistingOrders(userId!, multiDistribId),
    enabled,
    // Pas de stale-time long : on veut refléter rapidement les modifs serveur.
    staleTime: 30_000,
  });
}

/**
 * Récupère l'utilisateur courant via /api/user/me (compat). Le résultat est
 * mis en cache 5 min : appelé en parallèle des données shop.
 */
export function useShopMe() {
  return useQuery({
    queryKey: ["shop", "me"],
    queryFn: fetchShopMe,
    staleTime: 5 * 60_000,
  });
}

/**
 * Récupère et compose toutes les données du shop pour un MultiDistrib donné :
 * init (place / dates / vendors), categories (taxonomie), allProducts.
 * Construit aussi le `catalog` structuré attendu par les composants vues.
 */
export function useShopData(multiDistribId: number) {
  // Garde-fou : si l'URL ne contient pas un id numérique valide, on n'envoie
  // pas de requêtes (sinon /shop/init?multiDistrib=NaN renvoie une 400).
  const enabled = Number.isInteger(multiDistribId) && multiDistribId > 0;

  const init = useQuery({
    queryKey: ["shop", "init", multiDistribId],
    queryFn: () => fetchShopInit(multiDistribId),
    enabled,
  });

  const cats = useQuery({
    queryKey: ["shop", "categories", multiDistribId],
    queryFn: () => fetchShopCategories(multiDistribId),
    enabled,
  });

  const products = useQuery({
    queryKey: ["shop", "products", multiDistribId],
    queryFn: () => fetchShopProducts(multiDistribId),
    enabled,
  });

  const isLoading = init.isLoading || cats.isLoading || products.isLoading;
  const error = init.error ?? cats.error ?? products.error;

  let catalog: FilteredProductCatalog | null = null;
  let categoriesList: CategoryInfo[] = [];

  if (cats.data && products.data) {
    categoriesList = cats.data.categories;
    catalog = buildCatalog(categoriesList, products.data.products);
  }

  return {
    isLoading,
    error,
    init: init.data,
    categories: categoriesList,
    products: products.data?.products ?? [],
    catalog,
  };
}
