import type { PlaceInfo } from "../../types/shop";
import { DistributionDetails } from "./DistributionDetails";
import { CartButton } from "./CartButton";
import { COLORS } from "./theme";

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
  return (
    <header
      style={{
        backgroundColor: COLORS.white,
        borderBottom: "1px solid " + COLORS.lightGrey,
      }}
    >
      <div
        className="flex flex-wrap items-center gap-4"
        style={{
          maxWidth: 1240,
          margin: "auto",
          padding: "10px 16px",
        }}
      >
        <div style={{ flex: "1 1 320px", minWidth: 0 }}>
          <DistributionDetails startDate={startDate} endDate={endDate} place={place} />
        </div>

        <div style={{ flex: "1 1 280px", maxWidth: 460 }}>
          <label className="relative block">
            <i
              className="icon-search absolute"
              style={{
                left: 12,
                top: "50%",
                transform: "translateY(-50%)",
                color: COLORS.mediumGrey,
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
              style={{
                padding: "10px 12px 10px 36px",
                fontSize: "1rem",
                border: "1px solid " + COLORS.lightGrey,
                borderRadius: 4,
                outline: "none",
                backgroundColor: COLORS.white,
              }}
            />
          </label>
        </div>

        <div style={{ flex: "0 0 auto" }}>
          <CartButton onClick={onCartClick} />
        </div>
      </div>
    </header>
  );
}
