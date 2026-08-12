import type { ProductInfo } from "../../types/shop";
import { useCartStore } from "../../store/cart";
import { formatPrice, smartQty, pricePerUnit, formatNum } from "../../utils/format";
import { COLORS } from "./theme";
import { QuantityInput } from "./QuantityInput";

interface Props {
  product: ProductInfo;
  /** Affiche la ligne "X% de TVA inclue" (utilisé dans la modale). */
  displayVAT?: boolean;
}

/**
 * Bandeau bas d'une carte produit : qté + prix unitaire / prix total / bouton
 * commander. Port de react.store.ProductActions (Haxe).
 */
export function ProductActions({ product, displayVAT = false }: Props) {
  const add = useCartStore((s) => s.add);
  const setQuantity = useCartStore((s) => s.setQuantity);
  const qty = useCartStore((s) => s.quantityOf(product.id));

  const qtyLabel = smartQty(product.qt, product.unitType);
  const unitPriceLabel = pricePerUnit(product.price, product.qt, product.unitType);

  return (
    <div
      // marginTop:auto colle le bandeau au bas de la carte, quelle que soit la
      // longueur du nom ou du nombre de badges au-dessus. Les cartes d'une même
      // ligne étant étirées à la même hauteur par la grille, leurs bandeaux
      // s'alignent alors les uns avec les autres.
      className="flex items-start justify-between"
      style={{ padding: "0 10px 8px", marginTop: "auto" }}
    >
      {/* Quantité + prix unitaire.
          Toujours rendus, même vides : deux lignes de hauteur fixe garantissent
          que le prix et le bouton restent alignés d'une vignette à l'autre. */}
      <div className="flex flex-col text-left" style={{ minWidth: 0 }}>
        <span
          style={{
            fontWeight: 400,
            color: COLORS.darkGrey,
            fontSize: 22,
            lineHeight: 1.2,
            minHeight: "1.2em",
          }}
        >
          {qtyLabel}
        </span>
        <span
          style={{ color: COLORS.mediumGrey, fontSize: 14, minHeight: "1.2em" }}
        >
          {unitPriceLabel}
        </span>
      </div>

      {/* Prix total + ligne TVA optionnelle (modale produit) */}
      <div className="text-center" style={{ flex: "0 0 auto", padding: "0 8px" }}>
        <span style={{ fontWeight: 700, color: COLORS.third, fontSize: 22 }}>
          {formatPrice(product.price)}
        </span>
        {displayVAT && product.vat != null && product.vat !== 0 && (
          <div style={{ color: COLORS.mediumGrey, fontSize: 12, marginTop: 2 }}>
            {formatNum(product.vat)} % de TVA inclue
          </div>
        )}
      </div>

      {/* Rupture / stepper / bouton commander — ordre du legacy
          ProductActions.renderQuantityAction (Haxe) :
            stock <= 0       → "Rupture de stock"
            qty dans panier  → stepper -/qty/+
            sinon            → bouton sac */}
      {product.stock != null && product.stock <= 0 ? (
        <span
          style={{
            color: COLORS.third,
            fontSize: 14,
            fontWeight: 700,
            textAlign: "right",
            lineHeight: 1.15,
            flexShrink: 0,
          }}
        >
          Rupture
          <br />
          de stock
        </span>
      ) : qty > 0 ? (
        <QuantityInput
          value={qty}
          onChange={(v) => setQuantity(product.id, v)}
        />
      ) : (
        <button
          type="button"
          onClick={() => add(product, 1)}
          aria-label="Ajouter au panier"
          title="Ajouter au panier"
          className="flex items-center justify-center transition-colors"
          style={{
            width: 64,
            height: 40,
            borderRadius: 6,
            backgroundColor: COLORS.primary,
            color: COLORS.white,
            flexShrink: 0,
            border: "none",
            cursor: "pointer",
          }}
        >
          <i className="icon-basket-add" style={{ fontSize: 24 }} aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
