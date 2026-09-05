import type { PlaceInfo } from "../../types/shop";
import { DistributionDetails } from "./DistributionDetails";
import { CartButton } from "./CartButton";
import { COLORS, RADIUS } from "./theme";
import { useEcranEtroit } from "../../utils/useEcranEtroit";

interface Props {
  startDate: Date | null;
  endDate: Date | null;
  place: PlaceInfo | null;
  search: string;
  onSearch: (value: string) => void;
  onCartClick?: () => void;
}

/**
 * En-tête du shop : infos de distribution à gauche, recherche au centre,
 * pastille panier à droite. Port de react.store.Header (Haxe).
 */
export function Header({
  startDate,
  endDate,
  place,
  search,
  onSearch,
  onCartClick,
}: Props) {
  // Trois blocs de front — la distribution, la recherche, le panier — se
  // dépliaient en trois lignes sur un téléphone, soit près de deux cents
  // pixels collés en haut de l'écran. Sur petit écran ils tiennent en deux
  // rangées : ce qu'on lit d'un coup d'œil et le panier, puis la recherche.
  const etroit = useEcranEtroit();

  return (
    <header
      style={{
        backgroundColor: COLORS.blanc,
        borderBottom: `1px solid ${COLORS.trait}`,
      }}
    >
      <div
        className={etroit ? "flex flex-col" : "flex flex-wrap items-center gap-4"}
        style={{
          maxWidth: 1240,
          margin: "auto",
          padding: etroit ? "8px 12px 10px" : "10px 16px",
          gap: etroit ? 8 : undefined,
        }}
      >
        {etroit ? (
          <div className="flex items-center" style={{ gap: 10 }}>
            <div style={{ flex: "1 1 0", minWidth: 0 }}>
              <DistributionDetails startDate={startDate} endDate={endDate} place={place} />
            </div>
            <CartButton onClick={onCartClick} />
          </div>
        ) : (
          <div style={{ flex: "1 1 320px", minWidth: 0 }}>
            <DistributionDetails startDate={startDate} endDate={endDate} place={place} />
          </div>
        )}

        <div style={etroit ? { width: "100%" } : { flex: "1 1 280px", maxWidth: 460 }}>
          <label className="relative block">
            <i
              className="icon-search absolute"
              style={{
                left: 12,
                top: "50%",
                transform: "translateY(-50%)",
                color: COLORS.gris,
                fontSize: 16,
              }}
              aria-hidden="true"
            />
            <input
              type="search"
              value={search}
              onChange={(e) => onSearch(e.target.value)}
              placeholder="Recherche"
              className="w-full"
              onFocus={(e) => {
                e.currentTarget.style.borderColor = COLORS.vert;
                e.currentTarget.style.boxShadow = `0 0 0 3px ${COLORS.vertPale}`;
              }}
              onBlur={(e) => {
                e.currentTarget.style.borderColor = COLORS.trait;
                e.currentTarget.style.boxShadow = "none";
              }}
              style={{
                padding: "10px 12px 10px 36px",
                fontSize: "0.95rem",
                border: `1px solid ${COLORS.trait}`,
                borderRadius: RADIUS.bouton,
                outline: "none",
                backgroundColor: COLORS.blanc,
                transition: "border-color .13s ease, box-shadow .13s ease",
              }}
            />
          </label>
        </div>

        {!etroit && (
          <div style={{ flex: "0 0 auto" }}>
            <CartButton onClick={onCartClick} />
          </div>
        )}
      </div>
    </header>
  );
}
