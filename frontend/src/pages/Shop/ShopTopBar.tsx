import { useEffect, useRef, useState } from "react";
import { COLORS } from "./theme";
import type { ShopMe } from "../../api/shop";

interface Props {
  groupName: string;
  user?: ShopMe;
}

const linkStyle: React.CSSProperties = {
  color: "#888",
  fontSize: "1rem",
  textDecoration: "none",
  fontFamily: "Cabin, Arial, Helvetica, sans-serif",
  gap: 6,
  lineHeight: 1,
};

/**
 * Bandeau du haut du shop : nom du groupe en h1 italique à gauche, puis liens
 * Accueil / Aide et menu utilisateur à droite. Reproduit l'en-tête des autres
 * pages Go (cf. templates/design.html) afin que la SPA s'intègre visuellement
 * au site. Les liens pointent vers les pages Go (la SPA n'a pas encore son
 * propre /home).
 */
export function ShopTopBar({ groupName, user }: Props) {
  const userName = user ? `${user.firstName} ${user.lastName}`.trim() : "";

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
        <a href="/home" className="inline-flex items-center" style={linkStyle}>
          <i
            className="icon-chevron-left"
            style={{ fontSize: "0.85em" }}
            aria-hidden="true"
          />
          <span>Accueil</span>
        </a>

        <a href="#" className="inline-flex items-center" style={linkStyle}>
          <i
            className="icon-info"
            style={{ fontSize: "0.95em" }}
            aria-hidden="true"
          />
          <span>Aide</span>
        </a>

        {user && <UserMenu name={userName} user={user} />}
      </div>
    </div>
  );
}

/**
 * Menu déroulant ouvrant Déconnexion + admin DB éventuel. Ferme au clic
 * extérieur et à Escape. Implémentation maison plutôt que de réintroduire
 * Bootstrap dropdown.
 */
function UserMenu({ name, user }: { name: string; user: ShopMe }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} style={{ position: "relative" }}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        className="inline-flex items-center"
        style={{
          ...linkStyle,
          background: "transparent",
          border: "none",
          padding: 0,
          cursor: "pointer",
        }}
      >
        <span>{name}</span>
        <i
          className="icon-chevron-down"
          style={{ fontSize: "0.75em", marginLeft: 2 }}
          aria-hidden="true"
        />
      </button>
      {open && (
        <ul
          role="menu"
          style={{
            position: "absolute",
            right: 0,
            top: "calc(100% + 6px)",
            minWidth: 200,
            background: "#fff",
            border: "1px solid #ddd",
            borderRadius: 4,
            boxShadow: "0 4px 12px rgba(0,0,0,0.10)",
            listStyle: "none",
            margin: 0,
            padding: "4px 0",
            zIndex: 60,
          }}
        >
          {user.hasDatabaseAdmin && (
            <li role="none">
              <a
                href="/admin/db"
                role="menuitem"
                className="flex items-center"
                style={{
                  padding: "8px 14px",
                  color: "#333",
                  textDecoration: "none",
                  fontFamily: "Cabin, Arial, Helvetica, sans-serif",
                  gap: 8,
                  fontSize: "0.95rem",
                }}
              >
                <i className="icon-cog" aria-hidden="true" />
                <span>Base de données</span>
              </a>
            </li>
          )}
          <li role="none">
            <a
              href="/user/logout"
              role="menuitem"
              className="flex items-center"
              style={{
                padding: "8px 14px",
                color: "#333",
                textDecoration: "none",
                fontFamily: "Cabin, Arial, Helvetica, sans-serif",
                gap: 8,
                fontSize: "0.95rem",
              }}
            >
              <i className="icon-delete" aria-hidden="true" />
              <span>Déconnexion</span>
            </a>
          </li>
        </ul>
      )}
    </div>
  );
}
