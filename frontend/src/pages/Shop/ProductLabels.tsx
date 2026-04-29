import type { ProductInfo } from "../../types/shop";
import { COLORS } from "./theme";

interface Props {
  product: ProductInfo;
}

/**
 * Badges affichés sous le nom du produit : Bio, prix variable, vente en vrac,
 * vente en gros. Port de react.store.Labels (Haxe).
 */
export function ProductLabels({ product }: Props) {
  const labels: Array<{ icon: string; title: string }> = [];

  if (product.organic) {
    labels.push({ icon: "icon-bio", title: "Agriculture biologique" });
  }
  if (product.variablePrice) {
    labels.push({ icon: "icon-scale", title: "Prix variable selon pesée" });
  }
  if (product.bulk) {
    labels.push({ icon: "icon-bulk", title: "Vendu en vrac : pensez à prendre un contenant" });
  }
  if (product.wholesale) {
    labels.push({ icon: "icon-wholesale", title: "Ce produit est commandé en gros" });
  }

  if (labels.length === 0) return null;

  return (
    <span className="inline-flex items-center gap-2">
      {labels.map((l) => (
        <i
          key={l.icon}
          className={l.icon}
          title={l.title}
          aria-label={l.title}
          style={{ fontSize: 20, color: COLORS.mediumGrey }}
        />
      ))}
    </span>
  );
}
