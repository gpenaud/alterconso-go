import { useEffect } from "react";

/**
 * Met à jour document.title au format "<title> — Alterconso", aligné avec
 * le template Go `templates/base.html` ({{.Title}} — Alterconso). Restaure
 * l'ancien titre au démontage pour éviter qu'une page de transition
 * conserve un titre obsolète.
 */
export function useDocumentTitle(title: string | undefined | null) {
  useEffect(() => {
    if (!title) return;
    const previous = document.title;
    document.title = `${title} — Alterconso`;
    return () => {
      document.title = previous;
    };
  }, [title]);
}
