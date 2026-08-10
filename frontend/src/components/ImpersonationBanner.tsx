import { useEffect, useState } from "react";

/**
 * Bandeau « connecté en tant que », pendant SPA de celui rendu par
 * templates/design.html.
 *
 * Les pages servies par Go l'affichent depuis PageData.IsImpersonating. Les
 * routes SPA (/shop, /login, /profile, /groups) sont servies par
 * frontend/dist/index.html sans template : sans ce composant, l'utilisateur
 * perd l'avertissement ET le lien de retour dès qu'il entre dans le shop, sans
 * moyen évident de revenir à son propre compte.
 *
 * Le lien pointe vers /user/return, la même route que le bandeau Go — c'est
 * elle qui rétablit la session d'origine, côté serveur.
 */
interface SessionInfo {
  impersonating: boolean;
  userName?: string;
  impersonatorName?: string;
}

export function ImpersonationBanner() {
  const [session, setSession] = useState<SessionInfo | null>(null);

  useEffect(() => {
    // `credentials: include` : dans le shop, l'utilisateur est authentifié par
    // le cookie de session posé par les pages Go, pas par le jeton du store —
    // sans cette option la requête partirait anonyme et le bandeau ne
    // s'afficherait jamais.
    fetch("/api/session", { credentials: "include" })
      .then((r) => (r.ok ? r.json() : null))
      .then(setSession)
      .catch(() => setSession(null));
  }, []);

  if (!session?.impersonating) return null;

  return (
    <div
      style={{
        background: "#c0392b",
        color: "#fff",
        padding: "8px 14px",
        textAlign: "center",
        fontWeight: "bold",
      }}
    >
      Vous êtes connecté en tant que {session.userName}
      <a
        href="/user/return"
        style={{ color: "#fff", textDecoration: "underline", marginLeft: 14 }}
      >
        ← Revenir
        {session.impersonatorName ? ` en tant que ${session.impersonatorName}` : ""}
      </a>
    </div>
  );
}
