import type { CategoryInfo } from "../../types/shop";
import { COLORS, FONTS } from "./theme";
import { useEcranEtroit } from "../../utils/useEcranEtroit";

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
  // Douze catégories partagées à parts égales sur 375 px, cela fait trente
  // pixels chacune : ni l'image ni le mot n'étaient lisibles. Sur petit écran
  // la barre défile horizontalement, chaque catégorie gardant une largeur
  // qu'on peut viser du doigt.
  const etroit = useEcranEtroit();

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
        backgroundColor: COLORS.creme,
        borderBottom: `1px solid ${COLORS.trait}`,
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
          padding: etroit ? "0 8px" : "0 10px",
          height: etroit ? (compact ? "3.6em" : "5.4em") : compact ? "5em" : "9em",
          transition: "height 0.2s",
          // Le défilement horizontal, sans barre visible : sur un téléphone
          // c'est le doigt qui fait défiler, et une barre grise mangerait une
          // hauteur déjà comptée.
          overflowX: etroit ? "auto" : undefined,
          scrollbarWidth: etroit ? "none" : undefined,
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
              // La catégorie retenue se signale par un trait vert sous
              // elle, et non par un simple aplat un peu plus foncé que le
              // fond : on ne voyait pas laquelle était active.
              style={{
                // Largeur fixe et défilement sur petit écran ; partage à parts
                // égales sur grand, où tout tient de front.
                flex: etroit ? "0 0 auto" : "1 1 0",
                width: etroit ? 74 : undefined,
                minWidth: 0,
                background: isActive ? COLORS.vertPale : "transparent",
                borderTop: "none",
                borderLeft: "none",
                borderRight: "none",
                borderBottom: `3px solid ${isActive ? COLORS.vert : "transparent"}`,
                padding: 4,
                cursor: "pointer",
                color: isActive ? COLORS.vertFonce : COLORS.gris,
                fontFamily: FONTS.texte,
                textTransform: "uppercase",
                fontSize: etroit ? "0.62rem" : "0.7rem",
                lineHeight: etroit ? "0.78rem" : "0.9rem",
                transition: "background .13s ease, color .13s ease",
              }}
            >
              {item.image && (
                <img
                  src={item.image}
                  alt={item.name}
                  title={compact ? item.name : undefined}
                  style={{
                    height: compact ? "80%" : etroit ? "44%" : "50%",
                    width: "auto",
                    objectFit: "contain",
                    marginBottom: compact ? 0 : etroit ? 4 : 6,
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
                    height: etroit ? "1.6rem" : "1.8rem",
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
