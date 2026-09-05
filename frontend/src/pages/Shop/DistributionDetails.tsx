import type { PlaceInfo } from "../../types/shop";
import { hDate, hHour } from "../../utils/format";
import { COLORS } from "./theme";
import { useEcranEtroit } from "../../utils/useEcranEtroit";

interface Props {
  startDate: Date | null;
  endDate: Date | null;
  place: PlaceInfo | null;
}

/**
 * Bloc d'infos de la distribution : date+lieu sur la 1re ligne, horaire
 * début–fin sur la 2e. Port de react.store.DistributionDetails (Haxe).
 */
export function DistributionDetails({ startDate, endDate, place }: Props) {
  // Deux lignes — la date et le lieu, puis l'horaire — deviennent une seule
  // phrase sur un téléphone : chaque ligne gagnée en haut est une ligne de
  // produits gagnée en dessous.
  const etroit = useEcranEtroit();

  if (etroit) {
    return (
      <div style={{ color: COLORS.encre, fontSize: "0.88rem", lineHeight: 1.35 }}>
        <i
          className="icon-calendar"
          style={{ color: COLORS.gris, marginRight: "0.4rem", verticalAlign: "middle" }}
          aria-hidden="true"
        />
        <span>{hDate(startDate)}</span>
        {startDate && endDate && (
          <span style={{ color: COLORS.gris }}>
            {" · "}
            {hHour(startDate)}–{hHour(endDate)}
          </span>
        )}
        {place && (
          <span
            style={{
              display: "block",
              color: COLORS.gris,
              fontSize: "0.82rem",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            <i className="icon-map-marker" aria-hidden="true" style={{ marginRight: "0.25rem" }} />
            {place.name}
          </span>
        )}
      </div>
    );
  }

  return (
    <div
      style={{
        lineHeight: 1.5,
        padding: "10px 0",
        color: COLORS.encre,
      }}
    >
      <p style={{ margin: "0 0 0.2rem" }}>
        <i
          className="icon-calendar"
          style={{
            color: COLORS.gris,
            fontSize: "1em",
            verticalAlign: "middle",
            marginRight: "0.4rem",
          }}
          aria-hidden="true"
        />
        <span>Distribution le {hDate(startDate)}</span>
        {place && (
          <>
            {" à "}
            <i
              className="icon-map-marker"
              style={{
                color: COLORS.gris,
                fontSize: "1em",
                verticalAlign: "middle",
                marginRight: "0.2rem",
              }}
              aria-hidden="true"
            />
            <span>{place.name}</span>
          </>
        )}
      </p>
      {startDate && endDate && (
        <p style={{ margin: "0 0 0.2rem" }}>
          <i
            className="icon-clock"
            style={{
              color: COLORS.gris,
              fontSize: "1em",
              verticalAlign: "middle",
              marginRight: "0.4rem",
            }}
            aria-hidden="true"
          />
          <span>
            {hHour(startDate)} - {hHour(endDate)}
          </span>
        </p>
      )}
    </div>
  );
}
