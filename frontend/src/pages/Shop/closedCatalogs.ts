import { createContext, useContext, useMemo } from "react";

// Les catalogues dont la commande est close, au sein d'une même distribution.
//
// Un jour de distribution rassemble plusieurs producteurs, et chacun ferme
// quand il veut : la clôture se règle par catalogue (Distribution.OrderEndDate)
// et non pour le jour entier. Une distribution peut donc être ouverte tout en
// comptant un producteur déjà clos.
//
// Le serveur le dit déjà — /api/shop/init renvoie un « canOrder » par
// catalogue — mais le shop l'ignorait : tous les produits restaient
// commandables à l'écran, et la validation partait avec le catalogue clos dans
// le lot. Le serveur la refusait alors d'un 403, et comme le panier envoie une
// requête PAR catalogue, ce refus emportait toute la commande : les
// producteurs encore ouverts n'étaient jamais enregistrés.
//
// Le contexte plutôt que des props : « fermé ? » ne concerne que le bas d'une
// vignette produit, mais la vignette est trois niveaux sous la page. Faire
// descendre l'information de main en main obligerait CategorySection et
// ProductCard à porter une donnée dont ils n'ont que faire.
//
// Pas de composant « Provider » ici : un fichier qui exporte à la fois un
// composant et des hooks casse le rafraîchissement à chaud, et le linter le
// refuse. ShopPage pose donc <ClosedCatalogsContext.Provider> lui-même.
export const ClosedCatalogsContext = createContext<ReadonlySet<number>>(
  new Set(),
);

/** L'ensemble des catalogues clos de la distribution affichée. */
export function useClosedCatalogs(): ReadonlySet<number> {
  return useContext(ClosedCatalogsContext);
}

/**
 * Ce produit appartient-il à un catalogue clos ?
 *
 * Un catalogId absent — le cas d'un article de panier restauré depuis une
 * commande dont le produit a disparu du catalogue — est traité comme ouvert :
 * mieux vaut laisser le serveur trancher que griser à tort.
 */
export function useIsCatalogClosed(catalogId: number | undefined): boolean {
  const closed = useClosedCatalogs();
  return useMemo(
    () => (catalogId == null ? false : closed.has(catalogId)),
    [closed, catalogId],
  );
}
