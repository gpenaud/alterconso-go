import { COLORS, RADIUS } from "./theme";

export type TagFilter = "organic" | "bulk";

interface Props {
  active: Set<TagFilter>;
  onToggle: (tag: TagFilter) => void;
}

const ITEMS: Array<{ key: TagFilter; label: string; icon: string; activeColor: string }> = [
  { key: "organic", label: "Bio", icon: "icon-bio", activeColor: COLORS.vert },
  { key: "bulk", label: "Vrac", icon: "icon-bulk", activeColor: COLORS.alerte },
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
      <span
        style={{
          fontSize: "0.72rem",
          fontWeight: 700,
          letterSpacing: "0.07em",
          textTransform: "uppercase",
          color: COLORS.grisClair,
          marginRight: 4,
        }}
      >
        Filtrer
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
              padding: "6px 13px",
              borderRadius: RADIUS.rond,
              border: `1px solid ${isActive ? it.activeColor : COLORS.trait}`,
              background: isActive ? it.activeColor : COLORS.blanc,
              color: isActive ? COLORS.blanc : COLORS.gris,
              cursor: "pointer",
              fontWeight: isActive ? 700 : 500,
              transition: "all 0.13s ease",
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
