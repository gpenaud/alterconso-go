import type {
  CatalogCategory,
  ProductInfo,
  VendorInfo,
} from "../../types/shop";
import { ProductCard } from "./ProductCard";
import { COLORS, FONTS, RADIUS } from "./theme";
import { useEcranEtroit } from "../../utils/useEcranEtroit";

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
  const etroit = useEcranEtroit();
  const products: ProductInfo[] = category.subcategories.flatMap(
    (sc) => sc.products,
  );

  return (
    <section style={{ marginBottom: 24 }}>
      {/* Le titre de rubrique, dans la police et le brun des titres du site :
          il était en gris encre, du même poids que le nom des produits. */}
      <h2
        className="italic"
        style={{
          fontFamily: FONTS.titre,
          fontSize: etroit ? "1.3rem" : "1.75rem",
          fontWeight: 700,
          color: COLORS.titre,
          margin: 0,
          paddingBottom: 10,
          borderBottom: `1px solid ${COLORS.trait}`,
        }}
      >
        {category.info.name}
      </h2>

      {products.length === 0 ? (
        <div
          style={{
            backgroundColor: COLORS.creme,
            border: `1px solid ${COLORS.trait}`,
            borderRadius: RADIUS.carte,
            padding: etroit ? "22px 18px" : "32px 24px",
            margin: etroit ? "24px auto" : "56px auto 48px",
            textAlign: "center",
            maxWidth: 720,
            color: COLORS.gris,
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
