import { useEffect, useState } from "react";
import { useCartStore } from "../../store/cart";
import { submitOrder } from "../../api/shop";
import { formatPrice, smartQty } from "../../utils/format";
import { COLORS } from "./theme";
import { QuantityInput } from "./QuantityInput";

interface Props {
  onClose: () => void;
  /** Si défini, la commande est passée pour le compte de cet utilisateur (admin
   *  pour membre — cf. ?userId=N dans l'URL du shop). */
  targetUserId?: number;
}

/**
 * Drawer panier qui slide depuis la droite. Liste les produits du panier avec
 * miniature + nom + qté totale + sous-total, stepper et bouton supprimer ;
 * footer avec total + "Commander". Port libre de react.store.Cart +
 * CartDetails (Haxe), refondu en panneau latéral plutôt qu'en popover.
 */
export function CartPanel({ onClose, targetUserId }: Props) {
  const items = useCartStore((s) => s.items);
  const total = useCartStore((s) => s.total());
  const setQuantity = useCartStore((s) => s.setQuantity);
  const remove = useCartStore((s) => s.remove);
  const clear = useCartStore((s) => s.clear);
  const multiDistribId = useCartStore((s) => s.multiDistribId);

  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitted, setSubmitted] = useState(false);

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

  const submit = async () => {
    if (items.length === 0 || multiDistribId == null) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      // Le shop submit attend une commande par (multiDistrib, catalog) : si le
      // panier mélange plusieurs catalogues, on fait un appel par catalogue.
      const groups = new Map<number, typeof items>();
      for (const it of items) {
        if (!it.catalogId) {
          throw new Error(`Article "${it.name}" sans catalogue — panier corrompu`);
        }
        const arr = groups.get(it.catalogId) ?? [];
        arr.push(it);
        groups.set(it.catalogId, arr);
      }
      for (const [catalogId, arr] of groups) {
        await submitOrder(multiDistribId, {
          catalogId,
          userId: targetUserId,
          orders: arr.map((it) => ({
            productId: it.productId,
            qt: it.quantity,
          })),
        });
      }
      clear();
      setSubmitted(true);
    } catch (e) {
      setSubmitError((e as Error).message ?? "Erreur lors de la commande");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Panier"
      onClick={onClose}
      style={{
        position: "fixed",
        inset: 0,
        backgroundColor: "rgba(40, 28, 16, 0.45)",
        display: "flex",
        justifyContent: "flex-end",
        zIndex: 50,
      }}
    >
      <aside
        onClick={(e) => e.stopPropagation()}
        className="flex flex-col"
        style={{
          width: "100%",
          maxWidth: 420,
          height: "100%",
          backgroundColor: COLORS.white,
          boxShadow: "-8px 0 32px rgba(0,0,0,0.20)",
          animation: "slideInRight 0.22s ease-out",
        }}
      >
        {/* En-tête */}
        <div
          className="flex items-center justify-between"
          style={{
            padding: "16px 20px",
            borderBottom: `1px solid ${COLORS.lightGrey}`,
          }}
        >
          <div style={{ minWidth: 0 }}>
            <h2
              className="italic"
              style={{
                fontSize: "1.4rem",
                margin: 0,
                fontWeight: 400,
                color: COLORS.darkGrey,
              }}
            >
              {targetUserId ? "Panier" : "Mon panier"}
            </h2>
            {targetUserId && (
              <div
                style={{
                  fontSize: "0.75rem",
                  color: COLORS.mediumGrey,
                  marginTop: 2,
                }}
              >
                pour l'utilisateur #{targetUserId}
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Fermer"
            style={{
              width: 36,
              height: 36,
              borderRadius: "50%",
              border: "none",
              background: COLORS.bg2,
              color: COLORS.darkGrey,
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontSize: 16,
            }}
          >
            <i className="icon-delete" aria-hidden="true" />
          </button>
        </div>

        {/* Corps : liste */}
        <div style={{ flex: 1, overflowY: "auto", padding: 12 }}>
          {submitted ? (
            <div
              className="flex flex-col items-center justify-center text-center"
              style={{ height: "100%", padding: 24, color: COLORS.mediumGrey }}
            >
              <i
                className="icon-check"
                style={{
                  fontSize: 56,
                  color: COLORS.secondary,
                  marginBottom: 12,
                }}
                aria-hidden="true"
              />
              <p style={{ margin: 0, fontSize: "1.1rem", color: COLORS.darkGrey }}>
                Commande enregistrée !
              </p>
            </div>
          ) : items.length === 0 ? (
            <div
              className="flex flex-col items-center justify-center text-center"
              style={{ height: "100%", padding: 24, color: COLORS.mediumGrey }}
            >
              <i
                className="icon-basket"
                style={{ fontSize: 56, marginBottom: 12 }}
                aria-hidden="true"
              />
              <p style={{ margin: 0, fontSize: "1rem" }}>
                Votre panier est vide
              </p>
            </div>
          ) : (
            <ul style={{ listStyle: "none", margin: 0, padding: 0 }}>
              {items.map((it) => {
                const qtyTotalLabel = smartQty(
                  (it.qt ?? 0) * it.quantity,
                  it.unitType,
                );
                const lineTotal = it.price * it.quantity;
                return (
                  <li
                    key={it.productId}
                    style={{
                      backgroundColor: COLORS.bg2,
                      borderRadius: 6,
                      padding: 10,
                      marginBottom: 8,
                      display: "grid",
                      gridTemplateColumns: "56px 1fr",
                      gap: 10,
                      alignItems: "center",
                    }}
                  >
                    {it.image ? (
                      <img
                        src={it.image}
                        alt=""
                        style={{
                          width: 56,
                          height: 56,
                          objectFit: "cover",
                          borderRadius: 4,
                          display: "block",
                        }}
                      />
                    ) : (
                      <div
                        style={{
                          width: 56,
                          height: 56,
                          backgroundColor: "#f0eadb",
                          borderRadius: 4,
                        }}
                      />
                    )}

                    <div style={{ minWidth: 0 }}>
                      <div className="flex items-start justify-between" style={{ gap: 8 }}>
                        <div
                          style={{
                            fontSize: "0.9rem",
                            textTransform: "uppercase",
                            color: COLORS.darkGrey,
                            lineHeight: 1.2,
                            flex: 1,
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            display: "-webkit-box",
                            WebkitLineClamp: 2,
                            WebkitBoxOrient: "vertical",
                          }}
                        >
                          {it.name}
                        </div>
                        <button
                          type="button"
                          onClick={() => remove(it.productId)}
                          aria-label={`Retirer ${it.name}`}
                          style={{
                            border: "none",
                            background: "transparent",
                            color: COLORS.mediumGrey,
                            cursor: "pointer",
                            fontSize: 14,
                            padding: 4,
                            flexShrink: 0,
                          }}
                        >
                          <i className="icon-delete" aria-hidden="true" />
                        </button>
                      </div>

                      <div
                        className="flex items-center justify-between"
                        style={{ marginTop: 6, gap: 8 }}
                      >
                        <div style={{ fontSize: "0.85rem", color: COLORS.mediumGrey, minWidth: 0 }}>
                          {qtyTotalLabel && <span>{qtyTotalLabel}</span>}
                          {qtyTotalLabel && " · "}
                          <span style={{ color: COLORS.third, fontWeight: 700 }}>
                            {formatPrice(lineTotal)}
                          </span>
                        </div>
                        <QuantityInput
                          value={it.quantity}
                          onChange={(v) => setQuantity(it.productId, v)}
                        />
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {/* Footer */}
        {!submitted && items.length > 0 && (
          <div
            style={{
              borderTop: `1px solid ${COLORS.lightGrey}`,
              padding: "16px 20px",
              backgroundColor: COLORS.white,
            }}
          >
            <div
              className="flex items-center justify-between"
              style={{ marginBottom: 12 }}
            >
              <span style={{ fontSize: "0.95rem", color: COLORS.darkGrey }}>
                Total
              </span>
              <span
                style={{
                  fontSize: "1.4rem",
                  fontWeight: 700,
                  color: COLORS.third,
                }}
              >
                {formatPrice(total)}
              </span>
            </div>
            {submitError && (
              <div
                style={{
                  color: COLORS.third,
                  fontSize: "0.85rem",
                  marginBottom: 8,
                }}
              >
                {submitError}
              </div>
            )}
            <button
              type="button"
              onClick={submit}
              disabled={submitting}
              className="transition-colors"
              style={{
                width: "100%",
                padding: "12px 16px",
                background: COLORS.primary,
                color: COLORS.white,
                border: "none",
                borderRadius: 6,
                fontSize: "1rem",
                fontWeight: 700,
                textTransform: "uppercase",
                letterSpacing: "0.05em",
                cursor: submitting ? "not-allowed" : "pointer",
                opacity: submitting ? 0.7 : 1,
              }}
            >
              {submitting ? "Envoi…" : "Commander"}
            </button>
          </div>
        )}
      </aside>

      <style>{`
        @keyframes slideInRight {
          from { transform: translateX(100%); }
          to   { transform: translateX(0); }
        }
      `}</style>
    </div>
  );
}
