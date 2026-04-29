import type { CategoryInfo } from "../../types/shop";
import { COLORS } from "./theme";

interface Props {
  categories: CategoryInfo[];
  activeId: number | null;
  onSelect: (id: number | null) => void;
  /** Mode compact (sticky au scroll) : barre plus basse, labels masqués. */
  compact?: boolean;
}

const ALL_ID = 0;
const ALL_LABEL = "Tous les produits";
const ALL_IMAGE = "/img/taxo/allProducts.png";

/**
 * Barre de navigation horizontale des catégories taxonomiques. Inclut un
 * bouton "Tous les produits" (id 0) en première position. Port de
 * react.store.HeaderCategories (Haxe) : utilise les illustrations couleur.
 */
export function CategoryNav({ categories, activeId, onSelect, compact = false }: Props) {
  const items = [
    { id: ALL_ID, name: ALL_LABEL, image: ALL_IMAGE },
    ...categories.map((c) => ({
      id: c.id,
      name: c.name,
      image: c.image ?? "",
    })),
  ];

  return (
    <nav
      style={{
        backgroundColor: COLORS.bg2,
        textAlign: "center",
        textTransform: "uppercase",
        fontSize: "0.7rem",
        lineHeight: "0.9rem",
      }}
    >
      <div
        className="flex items-stretch"
        style={{
          maxWidth: 1240,
          margin: "auto",
          padding: "0 10px",
          height: compact ? "5em" : "9em",
          transition: "height 0.2s",
        }}
      >
        {items.map((item) => {
          const isActive =
            (item.id === ALL_ID && activeId == null) || item.id === activeId;
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => onSelect(item.id === ALL_ID ? null : item.id)}
              className="flex flex-col items-center justify-center transition-colors"
              style={{
                flex: "1 1 0",
                minWidth: 0,
                background: isActive ? COLORS.bg3 : "transparent",
                border: "none",
                padding: 4,
                cursor: "pointer",
                color: COLORS.darkGrey,
                textTransform: "uppercase",
                fontSize: "0.7rem",
                lineHeight: "0.9rem",
              }}
            >
              {item.image && (
                <img
                  src={item.image}
                  alt={item.name}
                  title={compact ? item.name : undefined}
                  style={{
                    height: compact ? "80%" : "50%",
                    width: "auto",
                    objectFit: "contain",
                    marginBottom: compact ? 0 : 6,
                  }}
                />
              )}
              {/* Label : caché en mode compact (legacy : `name = isSticky ? null : name`).
                   Sinon, ghost-text en gras (invisible) qui réserve largeur/hauteur,
                   + texte en absolute par-dessus → layout figé, bold sans déplacement. */}
              {!compact && (
                <span
                  style={{
                    position: "relative",
                    display: "block",
                    height: "1.8rem",
                    width: "100%",
                    textAlign: "center",
                  }}
                >
                  <span
                    aria-hidden="true"
                    style={{ visibility: "hidden", fontWeight: 700 }}
                  >
                    {item.name}
                  </span>
                  <span
                    style={{
                      position: "absolute",
                      inset: 0,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      fontWeight: isActive ? 700 : 400,
                    }}
                  >
                    {item.name}
                  </span>
                </span>
              )}
            </button>
          );
        })}
      </div>
    </nav>
  );
}
