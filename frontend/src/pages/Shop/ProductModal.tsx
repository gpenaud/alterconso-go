import { useEffect } from "react";
import type { ProductInfo, VendorInfo } from "../../types/shop";
import { ProductActions } from "./ProductActions";
import { ProductLabels } from "./ProductLabels";
import { COLORS } from "./theme";

interface Props {
  product: ProductInfo;
  vendor: VendorInfo;
  onClose: () => void;
}

/**
 * Modale de détail d'un produit. Composition visuelle inspirée des cartes
 * produits du shop : zone produit en haut sur fond crème, zone producteur en
 * bas séparée par un en-tête "Producteur" et fond blanc avec une bordure
 * crème pour cohérence avec la palette Alterconso.
 */
export function ProductModal({ product, vendor, onClose }: Props) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [onClose]);

  const portrait = vendor.images?.portrait ?? vendor.image ?? null;
  const hasLocation = !!(vendor.city || vendor.zipCode);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={product.name}
      onClick={onClose}
      style={{
        position: "fixed",
        inset: 0,
        backgroundColor: "rgba(40, 28, 16, 0.55)",
        backdropFilter: "blur(2px)",
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "center",
        zIndex: 50,
        overflowY: "auto",
        padding: "40px 16px",
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          backgroundColor: COLORS.white,
          borderRadius: 8,
          width: "100%",
          maxWidth: 680,
          position: "relative",
          boxShadow: "0 20px 60px rgba(0,0,0,0.30)",
          overflow: "hidden",
        }}
      >
        {/* Bouton fermeture flottant */}
        <button
          type="button"
          onClick={onClose}
          aria-label="Fermer"
          title="Fermer"
          className="transition-colors"
          style={{
            position: "absolute",
            top: 12,
            right: 12,
            width: 36,
            height: 36,
            borderRadius: "50%",
            border: "none",
            background: "rgba(255, 255, 255, 0.92)",
            color: COLORS.darkGrey,
            cursor: "pointer",
            fontSize: 16,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            boxShadow: "0 2px 6px rgba(0,0,0,0.12)",
            zIndex: 1,
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = COLORS.white;
            e.currentTarget.style.color = COLORS.primary;
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = "rgba(255, 255, 255, 0.92)";
            e.currentTarget.style.color = COLORS.darkGrey;
          }}
        >
          <i className="icon-delete" aria-hidden="true" />
        </button>

        {/* ─── Section produit ─── */}
        <div
          style={{ backgroundColor: COLORS.bg2, padding: 20 }}
          className="grid gap-5 grid-cols-1 md:grid-cols-12"
        >
          <div className="md:col-span-5">
            {product.image ? (
              <img
                src={product.image}
                alt=""
                style={{
                  width: "100%",
                  aspectRatio: "1 / 1",
                  objectFit: "cover",
                  display: "block",
                  borderRadius: 6,
                  boxShadow: "0 4px 16px rgba(0,0,0,0.10)",
                }}
              />
            ) : (
              <div
                style={{
                  width: "100%",
                  aspectRatio: "1 / 1",
                  backgroundColor: "#f0eadb",
                  borderRadius: 6,
                }}
              />
            )}
          </div>

          <div className="md:col-span-7 flex flex-col" style={{ gap: 10 }}>
            <span
              style={{
                fontSize: "0.8rem",
                color: COLORS.mediumGrey,
                fontStyle: "italic",
              }}
            >
              {vendor.name}
            </span>

            <h2
              style={{
                fontSize: "1.4rem",
                fontWeight: 400,
                textTransform: "uppercase",
                color: COLORS.darkGrey,
                margin: 0,
                lineHeight: 1.15,
              }}
            >
              {product.name}
            </h2>

            <div style={{ marginLeft: -3 }}>
              <ProductLabels product={product} />
            </div>

            {product.isResale && (
              <div
                style={{
                  backgroundColor: "#fff8e6",
                  border: "1px solid #f0d99a",
                  borderRadius: 6,
                  padding: "10px 12px",
                  fontSize: "0.9rem",
                  color: "#7a5b00",
                }}
              >
                <i className="icon-refresh" aria-hidden="true" style={{ marginRight: 6 }} />
                <b>Produit revendu</b>
                {product.resaleFrom && (
                  <>
                    {" — provient de "}
                    <span style={{ fontStyle: "italic" }}>{product.resaleFrom}</span>
                  </>
                )}
              </div>
            )}

            {product.desc && (
              <div
                style={{
                  color: COLORS.darkGrey,
                  lineHeight: 1.5,
                  fontSize: "0.9rem",
                }}
                dangerouslySetInnerHTML={{ __html: product.desc }}
              />
            )}

            <div
              style={{
                backgroundColor: COLORS.white,
                padding: "8px 4px",
                borderRadius: 6,
                marginTop: "auto",
                boxShadow: "0 2px 8px rgba(0,0,0,0.06)",
              }}
            >
              <ProductActions product={product} displayVAT />
            </div>
          </div>
        </div>

        {/* ─── Section producteur ─── */}
        <div style={{ padding: 20 }}>
          <div className="flex items-center" style={{ gap: 12, marginBottom: 12 }}>
            {portrait ? (
              <img
                src={portrait}
                alt={vendor.name}
                style={{
                  width: 56,
                  height: 56,
                  objectFit: "cover",
                  borderRadius: "50%",
                  border: `3px solid ${COLORS.bg2}`,
                  flexShrink: 0,
                }}
              />
            ) : (
              <div
                style={{
                  width: 56,
                  height: 56,
                  borderRadius: "50%",
                  backgroundColor: COLORS.bg2,
                  flexShrink: 0,
                }}
              />
            )}
            <div style={{ minWidth: 0 }}>
              <div
                style={{
                  fontSize: "0.65rem",
                  textTransform: "uppercase",
                  letterSpacing: "0.12em",
                  color: COLORS.mediumGrey,
                }}
              >
                Producteur
              </div>
              <div
                className="italic"
                style={{
                  fontSize: "1.15rem",
                  color: COLORS.darkGrey,
                  lineHeight: 1.2,
                }}
              >
                {vendor.name}
              </div>
            </div>
          </div>

          {(hasLocation || vendor.linkUrl) && (
            <div
              className="flex flex-wrap items-center"
              style={{
                backgroundColor: COLORS.bg2,
                borderRadius: 6,
                padding: "8px 12px",
                gap: "4px 16px",
                fontSize: "0.85rem",
                color: COLORS.darkGrey,
                marginBottom: 12,
              }}
            >
              {hasLocation && (
                <div className="flex items-center" style={{ gap: 6 }}>
                  <i
                    className="icon-map-marker"
                    style={{ color: COLORS.primary, fontSize: 14 }}
                    aria-hidden="true"
                  />
                  <span>
                    {vendor.city}
                    {vendor.zipCode && ` (${vendor.zipCode})`}
                  </span>
                </div>
              )}
              {vendor.linkUrl && (
                <div className="flex items-center" style={{ gap: 6 }}>
                  <i
                    className="icon-link"
                    style={{ color: COLORS.primary, fontSize: 14 }}
                    aria-hidden="true"
                  />
                  <a
                    href={vendor.linkUrl}
                    target="_blank"
                    rel="noreferrer"
                    style={{
                      color: COLORS.primary,
                      textDecoration: "none",
                      fontWeight: 600,
                    }}
                  >
                    {vendor.linkText || vendor.linkUrl}
                  </a>
                </div>
              )}
            </div>
          )}

          {vendor.desc && (
            <div
              style={{
                color: COLORS.darkGrey,
                lineHeight: 1.5,
                fontSize: "0.85rem",
              }}
              dangerouslySetInnerHTML={{ __html: vendor.desc }}
            />
          )}
        </div>
      </div>
    </div>
  );
}
