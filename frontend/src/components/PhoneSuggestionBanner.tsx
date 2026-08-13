import { useEffect, useState } from "react";

/**
 * Rappel « renseignez votre téléphone », pendant SPA de celui rendu par
 * templates/design.html.
 *
 * Même raison d'être que ImpersonationBanner : les routes servies par la SPA
 * (/shop, /profile, /groups) ne passent par aucun template Go, si bien que le
 * rappel s'arrêterait à la porte du shop — là où l'adhérent passe justement le
 * plus clair de son temps.
 *
 * L'information vient de /api/session, qui l'établit avec la même règle que les
 * pages Go (handler.suggestPhone).
 */
export function PhoneSuggestionBanner() {
  const [suggest, setSuggest] = useState(false);
  const [dismissed, setDismissed] = useState(
    () => sessionStorage.getItem("hidePhoneSuggestion") === "1",
  );

  useEffect(() => {
    // `credentials: include` : dans le shop, l'utilisateur est authentifié par
    // le cookie de session, pas par le jeton du store.
    fetch("/api/session", { credentials: "include" })
      .then((r) => (r.ok ? r.json() : null))
      .then((s) => setSuggest(Boolean(s?.suggestPhone)))
      .catch(() => setSuggest(false));
  }, []);

  if (!suggest || dismissed) return null;

  const hide = () => {
    sessionStorage.setItem("hidePhoneSuggestion", "1");
    setDismissed(true);
  };

  return (
    <div
      style={{
        background: "#e8f2fb",
        color: "#1c4e80",
        borderBottom: "1px solid #c3ddf2",
        padding: "8px 14px",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        gap: 10,
        flexWrap: "wrap",
      }}
    >
      <span>
        Vous n'avez pas encore renseigné de numéro de téléphone. Il permet de vous
        joindre en cas de problème pendant une distribution.
      </span>
      <a href="/account/edit" style={{ color: "#1c4e80", fontWeight: "bold" }}>
        Ajouter mon numéro
      </a>
      <button
        type="button"
        onClick={hide}
        aria-label="Masquer"
        style={{
          background: "none",
          border: "none",
          color: "#1c4e80",
          fontSize: 18,
          lineHeight: 1,
          cursor: "pointer",
          padding: "0 4px",
        }}
      >
        ×
      </button>
    </div>
  );
}
