import { useEffect } from "react";
import type { ProductInfo, VendorInfo } from "../../types/shop";
import { ProductActions } from "./ProductActions";
import { ProductLabels } from "./ProductLabels";
import { COLORS, FONTS, RADIUS } from "./theme";
import { useEcranEtroit } from "../../utils/useEcranEtroit";

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
  const etroit = useEcranEtroit();

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
        padding: etroit ? "12px 10px 24px" : "40px 16px",
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          backgroundColor: COLORS.blanc,
          borderRadius: RADIUS.panneau,
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
            color: COLORS.encre,
            cursor: "pointer",
            fontSize: 16,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            boxShadow: "0 2px 6px rgba(0,0,0,0.12)",
            zIndex: 1,
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = COLORS.blanc;
            e.currentTarget.style.color = COLORS.vert;
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = "rgba(255, 255, 255, 0.92)";
            e.currentTarget.style.color = COLORS.encre;
          }}
        >
          <i className="icon-delete" aria-hidden="true" />
        </button>

        {/* ─── Section produit ─── */}
        <div
          style={{ backgroundColor: COLORS.creme, padding: etroit ? 14 : 20 }}
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
                  borderRadius: RADIUS.bouton,
                  boxShadow: "0 4px 16px rgba(0,0,0,0.10)",
                }}
              />
            ) : (
              <div
                style={{
                  width: "100%",
                  aspectRatio: "1 / 1",
                  backgroundColor: COLORS.vide,
                  borderRadius: RADIUS.bouton,
                }}
              />
            )}
          </div>

          <div className="md:col-span-7 flex flex-col" style={{ gap: 10 }}>
            <span
              style={{
                fontSize: "0.8rem",
                color: COLORS.gris,
                fontStyle: "italic",
              }}
            >
              {vendor.name}
            </span>

            <h2
              style={{
                fontFamily: FONTS.titre,
                fontSize: "1.5rem",
                fontWeight: 700,
                color: COLORS.titre,
                margin: 0,
                lineHeight: 1.2,
              }}
            >
              {product.name}
            </h2>

            <div style={{ marginLeft: -3 }}>
              <ProductLabels product={product} />
            </div>

            {product.resaleFrom && (
              <div
                style={{
                  backgroundColor: COLORS.alertePale,
                  border: `1px solid ${COLORS.alerteTrait}`,
                  borderRadius: RADIUS.bouton,
                  padding: "10px 12px",
                  fontSize: "0.9rem",
                  color: COLORS.alerte,
                }}
              >
                <i className="icon-refresh" aria-hidden="true" style={{ marginRight: 6 }} />
                <b>Produit revendu</b> — provient de{" "}
                <span style={{ fontStyle: "italic" }}>{product.resaleFrom}</span>
              </div>
            )}

            {/* La description est saisie dans un simple textarea, côté
                administration : c'est du texte, et l'injecter comme du HTML
                laissait n'importe qui disposant des droits sur un catalogue
                placer un script dans la page de tous les adhérents. Les pages
                Go l'échappent depuis toujours ; ici, on ne garde que les
                retours à la ligne. */}
            {product.desc && (
              <div
                style={{
                  color: COLORS.encre,
                  lineHeight: 1.5,
                  fontSize: "0.9rem",
                  whiteSpace: "pre-wrap",
                }}
              >
                {product.desc}
              </div>
            )}

            <div
              style={{
                backgroundColor: COLORS.blanc,
                padding: "8px 4px",
                borderRadius: RADIUS.bouton,
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
                  border: `3px solid ${COLORS.creme}`,
                  flexShrink: 0,
                }}
              />
            ) : (
              <div
                style={{
                  width: 56,
                  height: 56,
                  borderRadius: "50%",
                  backgroundColor: COLORS.creme,
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
                  color: COLORS.gris,
                }}
              >
                Producteur
              </div>
              <div
                className="italic"
                style={{
                  fontSize: "1.15rem",
                  color: COLORS.encre,
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
                backgroundColor: COLORS.creme,
                borderRadius: RADIUS.bouton,
                padding: "8px 12px",
                gap: "4px 16px",
                fontSize: "0.85rem",
                color: COLORS.encre,
                marginBottom: 12,
              }}
            >
              {hasLocation && (
                <div className="flex items-center" style={{ gap: 6 }}>
                  <i
                    className="icon-map-marker"
                    style={{ color: COLORS.vert, fontSize: 14 }}
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
                    style={{ color: COLORS.vert, fontSize: 14 }}
                    aria-hidden="true"
                  />
                  <a
                    href={vendor.linkUrl}
                    target="_blank"
                    rel="noreferrer"
                    style={{
                      color: COLORS.vert,
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
                color: COLORS.encre,
                lineHeight: 1.5,
                fontSize: "0.85rem",
                whiteSpace: "pre-wrap",
              }}
            >
              {vendor.desc}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
