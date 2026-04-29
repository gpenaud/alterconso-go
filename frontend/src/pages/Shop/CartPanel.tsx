import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
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
  /** Catalogues sur lesquels l'utilisateur avait déjà commandé : utilisé pour
   *  envoyer des payloads vides à ceux dont tous les items ont été retirés
   *  (sans ça, le serveur garde l'ancienne commande pour ces catalogues). */
  existingCatalogIds?: number[];
}

/**
 * Drawer panier qui slide depuis la droite. Liste les produits du panier avec
 * miniature + nom + qté totale + sous-total, stepper et bouton supprimer ;
 * footer avec total + "Commander". Port libre de react.store.Cart +
 * CartDetails (Haxe), refondu en panneau latéral plutôt qu'en popover.
 */
export function CartPanel({ onClose, targetUserId, existingCatalogIds = [] }: Props) {
  const items = useCartStore((s) => s.items);
  const subtotal = useCartStore((s) => s.subtotal());
  const feesTotal = useCartStore((s) => s.feesTotal());
  const total = useCartStore((s) => s.total());
  const setQuantity = useCartStore((s) => s.setQuantity);
  const remove = useCartStore((s) => s.remove);
  const clear = useCartStore((s) => s.clear);
  const multiDistribId = useCartStore((s) => s.multiDistribId);

  const queryClient = useQueryClient();
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

  // Le panier peut être validé même vide si l'utilisateur avait déjà des
  // commandes — c'est l'équivalent d'une annulation (delete-then-insert avec
  // 0 ligne). On bloque seulement quand il n'y a rien à dire au serveur.
  const canSubmit = items.length > 0 || existingCatalogIds.length > 0;

  const submit = async () => {
    if (!canSubmit || multiDistribId == null) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      // Le shop submit attend une commande par (multiDistrib, catalog). On
      // groupe les items par catalogId et on inclut aussi les catalogues
      // existingCatalogIds qui n'ont plus d'items dans le panier — sinon le
      // serveur garde leurs anciennes commandes intactes.
      const groups = new Map<number, typeof items>();
      for (const it of items) {
        if (!it.catalogId) {
          throw new Error(`Article "${it.name}" sans catalogue — panier corrompu`);
        }
        const arr = groups.get(it.catalogId) ?? [];
        arr.push(it);
        groups.set(it.catalogId, arr);
      }
      for (const cid of existingCatalogIds) {
        if (!groups.has(cid)) groups.set(cid, []);
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
      // Invalide les caches dépendant de l'état serveur de la commande, sinon
      // une re-visite (shop pré-rempli, dashboard "Ma commande : X €") affiche
      // l'état d'avant submit pendant la durée de staleTime.
      queryClient.invalidateQueries({ queryKey: ["shop", "existingOrders"] });
      queryClient.invalidateQueries({ queryKey: ["home"] });
      setSubmitted(true);
      // Retour à /home après un bref affichage de la confirmation. Navigation
      // dure (window.location) car /home est une page Go hors SPA.
      window.setTimeout(() => {
        window.location.href = "/home";
      }, 800);
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
              {existingCatalogIds.length > 0 && (
                <p style={{ margin: "12px 0 0", fontSize: "0.85rem", color: COLORS.mediumGrey, maxWidth: 280 }}>
                  Tu peux confirmer pour annuler ta commande sur cette distribution.
                </p>
              )}
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

        {/* Footer
            Affiché dès qu'il y a quelque chose à dire au serveur :
              - panier non vide → bouton Commander, total
              - panier vide + commande existante → bouton Annuler ma commande */}
        {!submitted && canSubmit && (
          <div
            style={{
              borderTop: `1px solid ${COLORS.lightGrey}`,
              padding: "16px 20px",
              backgroundColor: COLORS.white,
            }}
          >
            {items.length > 0 && (
              <div style={{ marginBottom: 12 }}>
                {feesTotal > 0 && (
                  <>
                    <div
                      className="flex items-center justify-between"
                      style={{ fontSize: "0.85rem", color: COLORS.mediumGrey }}
                    >
                      <span>Sous-total</span>
                      <span>{formatPrice(subtotal)}</span>
                    </div>
                    <div
                      className="flex items-center justify-between"
                      style={{ fontSize: "0.85rem", color: COLORS.mediumGrey, marginTop: 2 }}
                    >
                      <span>Frais</span>
                      <span>{formatPrice(feesTotal)}</span>
                    </div>
                  </>
                )}
                <div
                  className="flex items-center justify-between"
                  style={{ marginTop: feesTotal > 0 ? 6 : 0 }}
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
              </div>
            )}
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
                background: items.length === 0 ? COLORS.third : COLORS.primary,
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
              {submitting
                ? "Envoi…"
                : items.length === 0
                ? "Annuler ma commande"
                : "Commander"}
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
