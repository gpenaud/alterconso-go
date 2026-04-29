import { useEffect, useRef, useState } from "react";
import { useCartStore } from "../../store/cart";
import { formatPrice } from "../../utils/format";
import { COLORS } from "./theme";

interface Props {
  onClick?: () => void;
}

/**
 * Pastille panier en haut à droite : icône violette + total + chevron.
 * Port de react.store.Cart (Haxe, version compacte sticky).
 */
export function CartButton({ onClick }: Props) {
  const total = useCartStore((s) => s.total());
  const count = useCartStore((s) => s.count());

  // Animation "bounce" sur l'icône quand le total augmente (legacy
  // Cart.hx::getDerivedStateFromProps + componentDidUpdate, classe `bounce`).
  const prevTotal = useRef(total);
  const [flash, setFlash] = useState(false);
  useEffect(() => {
    if (total > prevTotal.current) {
      setFlash(true);
      const id = window.setTimeout(() => setFlash(false), 750);
      return () => window.clearTimeout(id);
    }
    prevTotal.current = total;
  }, [total]);

  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center transition-shadow hover:shadow-sm"
      style={{
        // Largeur min stable même quand le total grandit (0 € → 1234.56 €)
        // pour que la barre de recherche du Header ne se décale pas.
        minWidth: 200,
        justifyContent: "space-between",
        backgroundColor: COLORS.white,
        border: "1px solid " + COLORS.lightGrey,
        borderRadius: 999,
        padding: "4px 8px 4px 4px",
        gap: 8,
        cursor: "pointer",
        color: COLORS.darkGrey,
      }}
    >
      <span
        className="relative flex items-center justify-center"
        style={{
          width: 36,
          height: 36,
          borderRadius: "50%",
          background: COLORS.primary,
          color: COLORS.white,
          animation: flash ? "cartBounce 0.6s ease" : undefined,
        }}
      >
        <i className="icon-basket" style={{ fontSize: 18 }} aria-hidden="true" />
        {count > 0 && (
          <span
            style={{
              position: "absolute",
              top: -4,
              right: -4,
              minWidth: 18,
              height: 18,
              padding: "0 5px",
              borderRadius: 9,
              background: COLORS.secondary,
              color: COLORS.white,
              fontSize: 11,
              fontWeight: 700,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              border: `2px solid ${COLORS.white}`,
            }}
          >
            {count}
          </span>
        )}
      </span>
      <span
        style={{
          fontWeight: 700,
          fontSize: "1.2rem",
          fontVariantNumeric: "tabular-nums",
        }}
      >
        {formatPrice(total)}
      </span>
      <i
        className="icon-chevron-down"
        style={{ color: COLORS.mediumGrey, fontSize: 14 }}
        aria-hidden="true"
      />
      <style>{`
        @keyframes cartBounce {
          0%   { transform: scale(1); }
          25%  { transform: scale(1.25); }
          50%  { transform: scale(0.92); }
          75%  { transform: scale(1.08); }
          100% { transform: scale(1); }
        }
      `}</style>
    </button>
  );
}
