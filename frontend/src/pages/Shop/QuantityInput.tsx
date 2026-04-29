import { COLORS } from "./theme";

interface Props {
  value: number;
  onChange: (qty: number) => void;
}

/**
 * Stepper -/qty/+ qui remplace le bouton "Ajouter au panier" quand le produit
 * est déjà dans le panier. Port de react.store.QuantityInput (Haxe).
 */
export function QuantityInput({ value, onChange }: Props) {
  const dec = () => onChange(Math.max(0, value - 1));
  const inc = () => onChange(value + 1);

  const stepStyle: React.CSSProperties = {
    flex: 1,
    backgroundColor: COLORS.primary,
    color: COLORS.white,
    fontSize: "1.4rem",
    lineHeight: 1,
    cursor: "pointer",
    textAlign: "center",
    padding: "6px 0",
    border: "none",
    userSelect: "none",
    transition: "background-color .2s",
  };

  return (
    <div
      className="flex items-stretch"
      style={{
        border: `1px solid ${COLORS.primary}`,
        borderRadius: 6,
        overflow: "hidden",
        width: 110,
        height: 40,
        flexShrink: 0,
      }}
    >
      <button
        type="button"
        aria-label="Diminuer la quantité"
        onClick={dec}
        style={stepStyle}
      >
        −
      </button>
      <div
        className="flex items-center justify-center"
        style={{
          flex: 1,
          backgroundColor: COLORS.white,
          color: COLORS.primary,
          fontSize: "1.1rem",
          fontWeight: 700,
        }}
      >
        {value}
      </div>
      <button
        type="button"
        aria-label="Augmenter la quantité"
        onClick={inc}
        style={stepStyle}
      >
        +
      </button>
    </div>
  );
}
