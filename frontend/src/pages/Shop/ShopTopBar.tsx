import { useEffect, useRef, useState } from "react";
import { COLORS, FONTS, RADIUS } from "./theme";
import { useEcranEtroit } from "../../utils/useEcranEtroit";
import type { ShopMe } from "../../api/shop";

interface Props {
  groupName: string;
  /** Adresse signée du logo du groupe, vide s'il n'en a pas. */
  logoUrl?: string;
  user?: ShopMe;
}

/** Les initiales de la pastille, comme dans l'en-tête des pages Go. */
function initiales(user: ShopMe) {
  const a = user.firstName?.trim()?.[0] ?? "";
  const b = user.lastName?.trim()?.[0] ?? "";
  return (a + b).toUpperCase() || "?";
}

/**
 * Bandeau du haut du shop : nom du groupe à gauche, retour aux commandes et
 * menu du compte à droite. Il reprend l'en-tête des pages Go (cf.
 * templates/design.html) pour qu'on ne sente pas la couture entre les deux
 * moitiés du site — la pastille aux initiales, les mêmes rubriques, la même
 * mise à l'écart de la déconnexion.
 */
export function ShopTopBar({ groupName, logoUrl, user }: Props) {
  // Sur un téléphone, cette barre coûtait deux à trois lignes avant qu'on ait
  // vu le moindre produit : logo de 58 px, nom du groupe en 1,9 rem, bouton de
  // retour en toutes lettres, puis la pastille. Elle se réduit à une ligne —
  // le logo et le nom, qui ramènent déjà aux commandes, et la pastille.
  const etroit = useEcranEtroit();

  return (
    <div
      className="flex flex-wrap items-center justify-between"
      style={{
        maxWidth: 1240,
        margin: "auto",
        padding: etroit ? "10px 12px 6px" : "14px 16px 10px",
        gap: etroit ? 10 : 16,
      }}
    >
      {/* Le même en-tête qu'à l'accueil : le logo cadré sur son dessin, puis
          le nom du groupe, l'ensemble ramenant aux commandes. Les valeurs de
          cadrage sont celles de `.ac-logo-groupe` dans www/css/alterconso.css
          — le fichier déposé entoure son dessin d'une large réserve blanche,
          et seul un fond permet de zoomer dedans pour ne garder que l'encre. */}
      <a
        href="/home"
        title="Retour aux commandes"
        className="flex items-center"
        style={{
          gap: logoUrl ? (etroit ? 10 : 22) : 0,
          minWidth: 0,
          flex: etroit ? "1 1 0" : undefined,
          textDecoration: "none",
          color: "inherit",
        }}
      >
        {logoUrl && (
          <span
            role="presentation"
            style={{
              flex: "none",
              display: "block",
              width: etroit ? 36 : 58,
              height: etroit ? 36 : 58,
              backgroundImage: `url('${logoUrl}')`,
              backgroundRepeat: "no-repeat",
              backgroundSize: "269%",
              backgroundPosition: "48.6% 53%",
              mixBlendMode: "multiply",
              filter: "saturate(.78)",
            }}
          />
        )}
        <h1
          style={{
            fontFamily: FONTS.titre,
            fontStyle: "normal",
            fontWeight: 700,
            fontSize: etroit ? "1.1rem" : "1.9rem",
            letterSpacing: "-.01em",
            color: COLORS.titre,
            margin: 0,
            minWidth: 0,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {groupName}
        </h1>
      </a>

      <div className="flex items-center" style={{ gap: 10, flexShrink: 0 }}>
        {/* Le lien « Aide » pointait sur href="#" : rien derrière. Le seul
            départ utile depuis le shop est le retour aux commandes — et sur un
            écran étroit, le logo et le nom du groupe y mènent déjà. */}
        {!etroit && (
        <a
          href="/home"
          className="inline-flex items-center"
          style={{
            gap: 7,
            padding: "8px 14px",
            borderRadius: RADIUS.bouton,
            border: `1px solid ${COLORS.trait}`,
            background: COLORS.blanc,
            color: COLORS.gris,
            fontFamily: FONTS.texte,
            fontSize: "0.9rem",
            fontWeight: 500,
            textDecoration: "none",
            lineHeight: 1.4,
          }}
        >
          <i className="icon-chevron-left" style={{ fontSize: "0.8em" }} aria-hidden="true" />
          <span>Retour aux commandes</span>
        </a>
        )}

        {user && <UserMenu user={user} groupName={groupName} />}
      </div>
    </div>
  );
}

/**
 * Menu du compte. Fermeture au clic extérieur et à Échap — implémentation
 * maison plutôt que de réintroduire le dropdown de Bootstrap, que la SPA ne
 * charge pas.
 */
function UserMenu({ user, groupName }: { user: ShopMe; groupName: string }) {
  const [open, setOpen] = useState(false);
  const etroit = useEcranEtroit();
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

  const entree: React.CSSProperties = {
    display: "flex",
    alignItems: "center",
    gap: 10,
    padding: "8px 12px",
    margin: "0 6px",
    borderRadius: 7,
    color: "#4a4a4a",
    textDecoration: "none",
    fontFamily: FONTS.texte,
    fontSize: "0.93rem",
  };

  return (
    <div ref={ref} style={{ position: "relative" }}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        className="inline-flex items-center"
        style={{
          gap: 8,
          background: open ? COLORS.creme : "transparent",
          border: "none",
          borderRadius: RADIUS.rond,
          padding: "4px 10px 4px 4px",
          cursor: "pointer",
          color: COLORS.encre,
          fontFamily: FONTS.texte,
        }}
      >
        <span
          className="inline-flex items-center justify-center"
          style={{
            width: 34,
            height: 34,
            borderRadius: "50%",
            background: COLORS.vert,
            color: COLORS.blanc,
            fontFamily: FONTS.titre,
            fontWeight: 700,
            fontSize: "0.82rem",
          }}
        >
          {initiales(user)}
        </span>
        {!etroit && (
          <span style={{ fontSize: "0.9rem", fontWeight: 600 }}>{user.firstName}</span>
        )}
        <i
          className="icon-chevron-down"
          style={{
            fontSize: "0.62em",
            color: COLORS.grisClair,
            transform: open ? "rotate(180deg)" : undefined,
            transition: "transform .13s ease",
          }}
          aria-hidden="true"
        />
      </button>

      {open && (
        <ul
          role="menu"
          style={{
            position: "absolute",
            right: 0,
            top: "calc(100% + 8px)",
            minWidth: etroit ? 236 : 268,
            maxWidth: "calc(100vw - 24px)",
            background: COLORS.blanc,
            border: `1px solid ${COLORS.trait}`,
            borderRadius: 12,
            boxShadow: "0 10px 32px rgba(0,0,0,.13)",
            listStyle: "none",
            margin: 0,
            padding: "6px 0",
            zIndex: 60,
          }}
        >
          <li role="none" style={{ display: "flex", alignItems: "center", gap: 11, padding: "10px 12px 12px" }}>
            <span
              className="inline-flex items-center justify-center"
              style={{
                width: 42,
                height: 42,
                borderRadius: "50%",
                background: COLORS.vert,
                color: COLORS.blanc,
                fontFamily: FONTS.titre,
                fontWeight: 700,
                fontSize: "0.95rem",
                flexShrink: 0,
              }}
            >
              {initiales(user)}
            </span>
            <span style={{ minWidth: 0 }}>
              <span
                style={{
                  display: "block",
                  fontFamily: FONTS.titre,
                  fontWeight: 700,
                  color: COLORS.titre,
                }}
              >
                {`${user.firstName} ${user.lastName}`.trim()}
              </span>
              <span
                style={{
                  display: "block",
                  fontSize: "0.8rem",
                  color: COLORS.grisClair,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {user.email}
              </span>
            </span>
          </li>

          <li
            role="none"
            style={{
              display: "flex",
              alignItems: "center",
              gap: 9,
              margin: "0 6px 6px",
              padding: "8px 11px",
              borderRadius: 8,
              background: COLORS.vertPale,
              color: COLORS.vertFonce,
              fontSize: "0.87rem",
              fontFamily: FONTS.texte,
            }}
          >
            <i className="icon-farmer" aria-hidden="true" style={{ flexShrink: 0 }} />
            <span
              style={{
                fontWeight: 700,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {groupName}
            </span>
          </li>

          <li role="none">
            <a href="/home" role="menuitem" style={entree}>
              <i className="icon-basket" aria-hidden="true" style={{ width: 17, textAlign: "center", color: COLORS.grisClair }} />
              <span>Mes commandes</span>
            </a>
          </li>
          <li role="none">
            <a href="/account" role="menuitem" style={entree}>
              <i className="icon-user" aria-hidden="true" style={{ width: 17, textAlign: "center", color: COLORS.grisClair }} />
              <span>Mon compte</span>
            </a>
          </li>
          {user.hasDatabaseAdmin && (
            <li role="none">
              <a href="/admin/db" role="menuitem" style={entree}>
                <i className="icon-cog" aria-hidden="true" style={{ width: 17, textAlign: "center", color: COLORS.grisClair }} />
                <span>Base de données</span>
              </a>
            </li>
          )}

          <li role="none" aria-hidden="true" style={{ height: 1, background: COLORS.trait, margin: "6px 0" }} />

          {/* Sortir n'est pas naviguer : l'entrée se distingue de celles du
              dessus, comme dans le menu des pages Go. */}
          <li role="none">
            <a href="/user/logout" role="menuitem" style={{ ...entree, color: COLORS.danger }}>
              <i className="icon-sign-out" aria-hidden="true" style={{ width: 17, textAlign: "center", color: "#b46b69" }} />
              <span>Déconnexion</span>
            </a>
          </li>
        </ul>
      )}
    </div>
  );
}
