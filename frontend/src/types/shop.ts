// Types issus du modèle legacy Haxe (Common.hx) — gardés au plus proche pour
// faciliter la migration et les tests pixel-parity.

export type Unit =
  | "Piece"
  | "Kilogram"
  | "Gram"
  | "Litre"
  | "Centilitre"
  | "Millilitre";

export interface PlaceInfo {
  id: number;
  name: string;
  address1?: string;
  address2?: string;
  zipCode?: string;
  city?: string;
  latitude?: number;
  longitude?: number;
}

export interface VendorInfo {
  id: number;
  name: string;
  desc: string;
  longDesc: string;
  image: string | null; // null = pas d'image (pas d'avatar fallback)
  profession: string;
  zipCode: string;
  city: string;
  linkText: string;
  linkUrl: string;
  images?: {
    logo?: string;
    portrait?: string;
    banner?: string;
    farm1?: string;
    farm2?: string;
    farm3?: string;
    farm4?: string;
  };
}

export interface CategoryInfo {
  id: number;
  name: string;
  image?: string; // URL absolue ou path "/img/taxo/..."
  displayOrder?: number;
  subcategories?: SubCategoryInfo[];
}

export interface SubCategoryInfo {
  id: number;
  name: string;
}

export interface ProductInfo {
  id: number;
  name: string;
  ref?: string | null;
  image?: string | null;
  price: number;
  vat?: number | null;
  vatValue?: number | null;
  desc?: string | null;
  categories: number[];
  subcategories: number[];
  orderable: boolean;
  stock?: number | null;
  hasFloatQt: boolean;
  qt?: number | null;
  unitType?: Unit | number | null;
  organic: boolean;
  variablePrice: boolean;
  wholesale: boolean;
  active: boolean;
  bulk: boolean;
  catalogId: number;
  catalogTax?: number | null;
  catalogTaxName?: string | null;
  vendorId?: number;
  distributionId?: number | null;
}

// ─── Réponses API ───────────────────────────────────────────────────────────

export interface ShopInitResponse {
  success: boolean;
  place: PlaceInfo;
  group: { id: number; name: string };
  distributionStartDate: string; // "YYYY-MM-DD HH:MM:SS"
  distributionEndDate: string;
  orderEndDates: Array<{ date: string; contracts: string[] }>;
  vendors: VendorInfo[];
  paymentInfos: string;
  catalogs: Array<{
    id: number;
    name: string;
    vendorId: number;
    vendor: { id: number; name: string };
    canOrder: boolean;
  }>;
  multiDistrib: { id: number; start: string; end: string; place: string };
}

export interface ShopCategoriesResponse {
  success: boolean;
  categories: CategoryInfo[];
}

export interface ShopAllProductsResponse {
  success: boolean;
  products: ProductInfo[];
}

// ─── Catalog "structuré" produit côté client (cf legacy makeCatalog) ────────

export interface CatalogSubCategory {
  info: SubCategoryInfo;
  products: ProductInfo[];
}

export interface CatalogCategory {
  info: CategoryInfo;
  subcategories: CatalogSubCategory[];
}

export interface FilteredProductCatalog {
  categories: CatalogCategory[];
  filter?: { search?: string | null; category?: number | null; subcategory?: number | null };
}
