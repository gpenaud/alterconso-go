import { COLORS } from "./theme";

export type TagFilter = "organic" | "bulk";

interface Props {
  active: Set<TagFilter>;
  onToggle: (tag: TagFilter) => void;
}

const ITEMS: Array<{ key: TagFilter; label: string; icon: string; activeColor: string }> = [
  { key: "organic", label: "Bio", icon: "icon-bio", activeColor: "#16993B" },
  { key: "bulk", label: "Vrac", icon: "icon-bulk", activeColor: "#a53fa1" },
];

/**
 * Chips de filtre par tag produit (Bio, Vrac). Le legacy Haxe avait laissé
 * cette fonctionnalité non implémentée (FilterUtil.filterProducts throw "To
 * implement"), on la finit ici à partir des flags booléens du modèle.
 */
export function ProductFilters({ active, onToggle }: Props) {
  return (
    <div
      className="flex flex-wrap items-center"
      style={{
        maxWidth: 1240,
        margin: "auto",
        padding: "8px 16px 0",
        gap: 8,
      }}
    >
      <span style={{ fontSize: "0.75rem", color: COLORS.mediumGrey, marginRight: 4 }}>
        Filtrer :
      </span>
      {ITEMS.map((it) => {
        const isActive = active.has(it.key);
        return (
          <button
            key={it.key}
            type="button"
            onClick={() => onToggle(it.key)}
            className="flex items-center"
            style={{
              gap: 6,
              fontSize: "0.85rem",
              padding: "4px 10px",
              borderRadius: 16,
              border: `1px solid ${isActive ? it.activeColor : COLORS.lightGrey}`,
              background: isActive ? it.activeColor : COLORS.white,
              color: isActive ? COLORS.white : COLORS.darkGrey,
              cursor: "pointer",
              fontWeight: isActive ? 700 : 400,
              transition: "all 0.15s",
            }}
          >
            <i className={it.icon} style={{ fontSize: 14 }} aria-hidden="true" />
            {it.label}
          </button>
        );
      })}
    </div>
  );
}
