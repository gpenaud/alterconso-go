// Port de FilterUtil.makeCatalog (Haxe) : construit l'arbre Catalog
// (categories → subcategories → products) à partir d'une liste plate de
// CategoryInfo (avec subcategories embarquées) et la liste de ProductInfo.

import type {
  CategoryInfo,
  ProductInfo,
  CatalogCategory,
  FilteredProductCatalog,
} from "../types/shop";

export function buildCatalog(
  categories: CategoryInfo[],
  products: ProductInfo[],
): FilteredProductCatalog {
  // Index des CategoryInfo par id pour lookup rapide.
  const catById = new Map<number, CategoryInfo>();
  const subById = new Map<number, { id: number; name: string }>();
  for (const c of categories) {
    catById.set(c.id, c);
    for (const sc of c.subcategories ?? []) {
      subById.set(sc.id, sc);
    }
  }

  // Cache CatalogCategory et CatalogSubCategory par id pour préserver
  // l'ordre d'apparition (premier produit qui matche la catégorie/sub-cat).
  const catalogCatById = new Map<number, CatalogCategory>();
  const catalog: FilteredProductCatalog = { categories: [] };

  for (const p of products) {
    const catId = p.categories[0];
    const subcatId = p.subcategories[0];
    if (catId == null) continue;

    let cat = catalogCatById.get(catId);
    if (!cat) {
      const info = catById.get(catId);
      if (!info) continue; // catégorie référencée par le produit mais inexistante
      cat = { info, subcategories: [] };
      catalogCatById.set(catId, cat);
      catalog.categories.push(cat);
    }

    let subcat = cat.subcategories.find((sc) => sc.info.id === subcatId);
    if (!subcat) {
      const subInfo = subById.get(subcatId);
      if (!subInfo) continue;
      subcat = { info: subInfo, products: [] };
      cat.subcategories.push(subcat);
    }
    subcat.products.push(p);
  }

  // Tri par displayOrder puis nom — comme le legacy.
  catalog.categories.sort((a, b) => {
    const oa = a.info.displayOrder ?? 999;
    const ob = b.info.displayOrder ?? 999;
    if (oa !== ob) return oa - ob;
    return a.info.name.localeCompare(b.info.name);
  });

  return catalog;
}

export function filterCatalog(
  catalog: FilteredProductCatalog,
  filter: { search?: string | null; category?: number | null },
): FilteredProductCatalog {
  let cats = catalog.categories;
  if (filter.category != null && filter.category !== 0) {
    cats = cats.filter((c) => c.info.id === filter.category);
  }
  if (filter.search) {
    const q = filter.search.toLowerCase();
    cats = cats
      .map((c) => ({
        info: c.info,
        subcategories: c.subcategories
          .map((sc) => ({
            info: sc.info,
            products: sc.products.filter((p) => p.name.toLowerCase().includes(q)),
          }))
          .filter((sc) => sc.products.length > 0),
      }))
      .filter((c) => c.subcategories.length > 0);
  }
  return { categories: cats, filter };
}
