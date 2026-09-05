import { useCallback, useSyncExternalStore } from "react";

/**
 * Vrai sur les écrans étroits.
 *
 * Les styles du shop sont écrits en ligne — c'est le parti pris du portage
 * depuis le Haxe — et aucune requête média ne peut les atteindre. C'est donc
 * le composant qui choisit, et pour choisir il doit savoir sur quoi il
 * s'affiche.
 *
 * 640 px : la limite `sm` de Tailwind, déjà employée par la grille des
 * produits. Deux seuils différents auraient fait basculer la page en deux
 * temps, avec un entre-deux que personne n'aurait dessiné.
 *
 * `useSyncExternalStore` plutôt qu'un effet qui appelle setState : c'est
 * l'abonnement à une source extérieure, et l'écrire ainsi évite le rendu en
 * cascade que React déconseille.
 */
export function useEcranEtroit(seuil = 640): boolean {
  const requete = `(max-width: ${seuil}px)`;

  const abonner = useCallback(
    (prevenir: () => void) => {
      const mq = window.matchMedia(requete);
      mq.addEventListener("change", prevenir);
      return () => mq.removeEventListener("change", prevenir);
    },
    [requete],
  );

  const lire = useCallback(() => window.matchMedia(requete).matches, [requete]);

  // Au rendu serveur — inexistant ici, mais l'API l'exige — on suppose le
  // grand écran : c'est la mise en page de repli du shop.
  return useSyncExternalStore(abonner, lire, () => false);
}
