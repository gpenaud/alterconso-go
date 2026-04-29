import type {
  CatalogCategory,
  ProductInfo,
  VendorInfo,
} from "../../types/shop";
import { ProductCard } from "./ProductCard";
import { COLORS } from "./theme";

interface Props {
  category: CatalogCategory;
  vendors: VendorInfo[];
  onProductClick?: (product: ProductInfo, vendor?: VendorInfo) => void;
}

/**
 * Section "Fruits et légumes" + grille de produits. Affiche un encart "Aucun
 * produit dans la catégorie" si vide. Port de react.store.ProductListCategory
 * (sans le titre "Tous" de sous-catégorie qu'on cache).
 */
export function CategorySection({ category, vendors, onProductClick }: Props) {
  const products: ProductInfo[] = category.subcategories.flatMap(
    (sc) => sc.products,
  );

  return (
    <section style={{ marginBottom: 24 }}>
      <h2
        className="italic"
        style={{
          fontSize: "2rem",
          fontWeight: 400,
          color: COLORS.darkGrey,
          margin: 0,
        }}
      >
        {category.info.name}
      </h2>

      {products.length === 0 ? (
        <div
          style={{
            backgroundColor: COLORS.bg2,
            border: "1px solid #E8DFC6",
            borderRadius: 8,
            padding: "32px 24px",
            margin: "56px auto 48px",
            textAlign: "center",
            maxWidth: 720,
            color: COLORS.mediumGrey,
          }}
        >
          <p style={{ fontSize: "1.25rem", margin: 0 }}>
            Il n'y a aucun produit dans la catégorie « {category.info.name} »
          </p>
        </div>
      ) : (
        <div
          className="grid gap-4 grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4"
          style={{ marginTop: 16 }}
        >
          {products.map((p) => {
            const vendor = vendors.find((v) => v.id === p.vendorId);
            return (
              <ProductCard
                key={p.id}
                product={p}
                vendor={vendor}
                onClick={onProductClick}
              />
            );
          })}
        </div>
      )}
    </section>
  );
}
