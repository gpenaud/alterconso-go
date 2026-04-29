import type { ProductInfo, VendorInfo } from "../../types/shop";
import { ProductLabels } from "./ProductLabels";
import { ProductActions } from "./ProductActions";
import { COLORS } from "./theme";

interface Props {
  product: ProductInfo;
  vendor?: VendorInfo;
  onClick?: (product: ProductInfo, vendor?: VendorInfo) => void;
}

/**
 * Carte produit : image (ou fallback catégorie) + nom + producteur + badges +
 * bandeau bas (qté/prix/commander). Port de react.store.Product (Haxe).
 */
export function ProductCard({ product, vendor, onClick }: Props) {
  const lowStock =
    product.stock != null && product.stock > 0 && product.stock <= 10;
  // Avatar producteur (legacy farmerAvatar) : portrait dédié sinon image générique.
  const farmerAvatar = vendor?.images?.portrait ?? vendor?.image ?? null;

  return (
    <div
      style={{
        backgroundColor: COLORS.bg2,
        borderRadius: 4,
        overflow: "hidden",
        boxShadow: "none",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <button
        type="button"
        onClick={() => onClick?.(product, vendor)}
        className="text-left transition-shadow hover:shadow-sm"
        style={{
          background: "transparent",
          border: "none",
          padding: 0,
          cursor: onClick ? "pointer" : "default",
          width: "100%",
        }}
      >
        {/* Image + avatar producteur en superposition (legacy farmerAvatar :
             position absolute bas-droite, masqué en xs). */}
        <div className="relative">
          {product.image ? (
            <img
              src={product.image}
              alt=""
              className="block w-full h-[120px] md:h-[240px] object-cover"
            />
          ) : (
            <div
              className="block w-full h-[120px] md:h-[240px]"
              style={{ backgroundColor: "#f0eadb" }}
            />
          )}
          {farmerAvatar && (
            <img
              src={farmerAvatar}
              alt={vendor?.name ?? ""}
              title={vendor?.name}
              className="hidden sm:block absolute object-cover"
              style={{
                width: 60,
                height: 60,
                bottom: -20,
                right: 12,
                borderRadius: "50%",
                border: `3px solid ${COLORS.white}`,
                backgroundColor: "#ededed",
                boxShadow: "0 2px 6px rgba(0,0,0,0.15)",
              }}
            />
          )}
        </div>

        {/* Contenu texte — padding-right étendu (sm+) pour ne pas chevaucher
             l'avatar producteur qui dépasse sur le contenu. */}
        <div
          style={{ padding: 10 }}
          className={farmerAvatar ? "sm:!pr-20" : undefined}
        >
          {/* Nom (h3 uppercase) */}
          <h3
            style={{
              fontSize: "1.08rem",
              lineHeight: "normal",
              fontStyle: "normal",
              textTransform: "uppercase",
              marginBottom: 3,
              fontWeight: 400,
              maxHeight: 40,
              overflow: "hidden",
              color: COLORS.darkGrey,
              margin: "0 0 3px 0",
            }}
          >
            {product.name}
          </h3>

          {/* Producteur + revente + low stock */}
          <p
            style={{
              fontSize: "0.9rem",
              color: COLORS.mediumGrey,
              marginBottom: 0,
              maxHeight: 65,
              overflow: "hidden",
              margin: 0,
            }}
          >
            {vendor && <span>{vendor.name}</span>}
            {product.resaleFrom && (
              <>
                <i
                  className="icon-refresh"
                  title="Produit revendu"
                  aria-label="Produit revendu"
                  style={{ margin: "0 6px", fontSize: 12 }}
                />
                <span style={{ fontStyle: "italic" }}>{product.resaleFrom}</span>
              </>
            )}
            {lowStock && (
              <span
                style={{
                  display: "block",
                  color: COLORS.third,
                  marginTop: 2,
                }}
              >
                Plus que {product.stock} en stock
              </span>
            )}
          </p>

          {/* Badges (Bio, prix variable, etc.) */}
          <p style={{ marginLeft: -3, margin: "4px 0 0 -3px" }}>
            <ProductLabels product={product} />
          </p>
        </div>
      </button>

      {/* Bandeau bas (sortie de la zone cliquable pour ne pas déclencher l'open
          sur clic du bouton de commande) */}
      <ProductActions product={product} />
    </div>
  );
}
