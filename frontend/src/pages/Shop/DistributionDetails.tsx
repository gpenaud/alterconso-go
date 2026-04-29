import type { PlaceInfo } from "../../types/shop";
import { hDate, hHour } from "../../utils/format";
import { COLORS } from "./theme";

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
  return (
    <div
      style={{
        lineHeight: 1.5,
        padding: "10px 0",
        color: COLORS.darkGrey,
      }}
    >
      <p style={{ margin: "0 0 0.2rem" }}>
        <i
          className="icon-calendar"
          style={{
            color: COLORS.mediumGrey,
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
                color: COLORS.mediumGrey,
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
              color: COLORS.mediumGrey,
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
