import { COLORS } from "./theme";

interface Props {
  groupName: string;
  userName?: string;
}

/**
 * Bandeau du haut du shop : nom du groupe en h1 italique à gauche, lien
 * "Accueil" + nom de l'utilisateur à droite. Reproduit l'en-tête des autres
 * pages Go (cf. templates/design.html) afin que la SPA s'intègre visuellement
 * au site. Les liens pointent vers les pages Go (la SPA n'a pas encore son
 * propre /home).
 */
export function ShopTopBar({ groupName, userName }: Props) {
  return (
    <div
      className="flex flex-wrap items-center justify-between"
      style={{
        maxWidth: 1240,
        margin: "auto",
        padding: "10px 16px 0",
        gap: 16,
      }}
    >
      <h1
        style={{
          fontStyle: "italic",
          fontFamily: "Cabin, Arial, Helvetica, sans-serif",
          fontWeight: 400,
          fontSize: "2.25rem",
          color: COLORS.darkGrey,
          margin: 0,
          minWidth: 0,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {groupName}
      </h1>

      <div className="flex items-center" style={{ gap: 16, flexShrink: 0 }}>
        <a
          href="/home"
          className="inline-flex items-center"
          style={{
            color: "#888",
            fontSize: "1rem",
            textDecoration: "none",
            fontFamily: "Cabin, Arial, Helvetica, sans-serif",
            gap: 6,
            lineHeight: 1,
          }}
        >
          <i
            className="icon-chevron-left"
            style={{ fontSize: "0.85em" }}
            aria-hidden="true"
          />
          <span>Accueil</span>
        </a>
        {userName && (
          <span
            style={{
              color: "#888",
              fontSize: "1rem",
              fontFamily: "Cabin, Arial, Helvetica, sans-serif",
              lineHeight: 1,
            }}
          >
            {userName}
          </span>
        )}
      </div>
    </div>
  );
}
