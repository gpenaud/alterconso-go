/**
 * Les jetons du shop, alignés sur la charte du site.
 *
 * Ils reprenaient tels quels ceux de Cagette — violet #a53fa1, vert #84BD55,
 * orange #E95219 — trois couleurs qu'on ne trouve nulle part ailleurs sur le
 * site : passer de la page des commandes au shop donnait le sentiment de
 * changer d'application. Ceux-ci sont la copie de la palette `.cyc` de
 * templates/cycles_style.html et de `:root` dans www/css/alterconso.css.
 *
 * La règle est celle du reste de l'interface : le vert ne dit que ce qui
 * s'actionne et ce qui va bien, le brun porte les titres et les montants,
 * l'ocre et le rouge signalent les ennuis. Un prix n'est pas une alerte : il
 * était orange vif, de la même couleur qu'une rupture de stock.
 */
export const COLORS = {
  vert: "#5a8a00",
  vertFonce: "#456a00",
  vertPale: "#f2f7e8",

  /** Brun terre : titres, noms de produits, montants. */
  titre: "#583816",

  /** Ce qui empêche : rupture de stock, échec d'envoi. */
  danger: "#8f302e",
  dangerPale: "#fdf1f0",
  /** Ce qui avertit sans bloquer : stock bas, commande pour autrui. */
  alerte: "#8a5a1c",
  alertePale: "#fdf3e6",
  alerteTrait: "#e8d2ae",

  blanc: "#ffffff",
  creme: "#fbf9f4",
  trait: "#e7e3d9",
  encre: "#26251f",
  gris: "#6d6a63",
  grisClair: "#9a968e",
  /** Le gris d'une image absente — assez chaud pour ne pas trouer la page. */
  vide: "#efece3",
};

/**
 * Cabin pour les titres seulement, comme sur le reste du site : le shop
 * l'imposait à tout, y compris aux paragraphes et aux boutons.
 */
export const FONTS = {
  texte:
    '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
  titre: 'Cabin, -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif',
};

/** Les rayons du site : 6 pour les commandes, 12–14 pour les cartes. */
export const RADIUS = {
  bouton: 8,
  carte: 12,
  panneau: 14,
  rond: 999,
};

/** L'ombre portée des panneaux du site — discrète, deux couches. */
export const OMBRE = "0 1px 2px rgba(0,0,0,.05), 0 1px 12px rgba(0,0,0,.03)";
