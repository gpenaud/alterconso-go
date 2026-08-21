# Refonte du front-end — décisions

Maquettes : canevas « Tunnel de commande Alterconso » (30 écrans, cinq pages) —
https://claude.ai/code/artifact/e7e103c9-6da4-44f9-b817-a02f86346da1
Le canevas s'édite dans le navigateur et contient ses propres sources.

Branche : `refonte-front`. Routes de la refonte préfixées par `/refonte`, à
côté de l'existant : rien n'est remplacé tant que la bascule n'est pas décidée.

## Le principe

Toutes les fonctionnalités actuelles sont conservées ; c'est le **parcours** qui
change. Un écran pose une question, et une seule. L'accueil ne demande plus de
choisir un groupe, une distribution, un catalogue : il annonce la prochaine
distribution et propose de commander.

Les écrans d'administration suivent une autre règle : densité, tableaux,
filtres. Même palette, autre grammaire.

## Typographie

**Alegreya** pour les titres et les chiffres, **Cabin** pour le texte courant.
Les deux sous licence SIL OFL, donc redistribuables avec l'application.

Cabin ne disparaît pas : elle change de rôle. C'est la police actuelle de
l'application, et la garder en texte courant conserve un fil avec ce que les
adhérents connaissent.

Les fontes sont **auto-hébergées** (`public/fonts`, 180 Ko après
déduplication : Google sert des fontes variables, plusieurs poids partageant le
même fichier). Deux raisons : la page ne dépend plus d'un tiers pour s'afficher,
et l'adresse IP de chaque adhérent cesse d'être transmise à Google au seul motif
d'afficher du texte.

## Couleurs

Nommées par **rôle**, jamais par teinte — `surface` et `control` disent quoi en
faire, « vert clair » et « vert foncé » non. C'est ce qui sépare une interface où
tout est vert et où rien ne ressort, d'une où l'œil sait immédiatement où
cliquer.

| Token | Valeur | Emploi |
|---|---|---|
| `canvas` | `#faf3e4` | fond de page |
| `card` | `#fffaf0` | blocs posés dessus |
| `surface` | `#a9cd82` | bandeaux, en-têtes, confirmation |
| `surface-deep` | `#2c4423` | navigation de l'administration |
| `control` | `#4c7c3c` | boutons secondaires, pastilles, cases cochées |
| `action` | `#c1440e` | action principale — **une seule par écran** |
| `ink` / `ink-muted` / `ink-faint` | `#22271e` / `#6b6f66` / `#a3a79c` | texte |

Sur `surface`, le texte est en `ink` et jamais en crème : le contraste tombe
sous le seuil de lisibilité en plein soleil, sur un parking de salle des fêtes —
qui est le contexte d'usage réel.

Les tokens `ac-*` de l'ancienne charte restent définis : 31 classes les
utilisent encore dans huit pages. Les retirer ne casserait pas la compilation,
Tailwind ignorant une classe inconnue, mais dégraderait l'affichage sans
avertissement. À supprimer quand le dernier écran sera migré.

## Ordre de migration

**Le parcours adhérent d'abord**, l'administration ensuite.

La SPA couvre déjà la boutique, donc le chemin est court ; c'est là que le
tunnel change quelque chose pour 61 personnes ; et l'administration actuelle
fonctionne — la refaire en premier serait du confort pour l'équipe au prix d'un
risque pour les adhérents.

1. Socle : tokens, fontes, composants de base — **fait**
2. Accueil (`/refonte`), choix de distribution (`/refonte/distributions`) — **fait**
3. Mes commandes (`/refonte/commandes`), compte (`/refonte/compte`) — **fait**
4. Boutique et panier : raccordés à l'existant (`/shop/:id`), non réécrits
5. Confirmation de commande (`/refonte/confirmation/:id`) — **fait**, mais non
   raccordée : la boutique affiche « Commande enregistrée ! » dans son panneau
   et n'y redirige pas. Le raccord touche `CartPanel`, qui tourne en production
   sur `/shop/:id` — il attendra la bascule plutôt que de modifier un flux
   utilisé aujourd'hui.
6. Fiche producteur, messagerie adhérent — reste à faire
7. Administration : tableau de bord, commandes, distributions, catalogues
8. Membres, adhésions, droits, messagerie
9. Retrait des tokens `ac-*` et des templates Go correspondants

### Ajouts côté serveur imposés par le parcours

Le tunnel demandait des données que l'API ne rendait pas :

- `orderStartAt` / `orderEndAt` : les instants bruts. L'API ne renvoyait que des
  dates mises en forme, dont une interface ne peut rien calculer — elle ne peut
  que les recopier et laisser le lecteur faire la soustraction.
- `vendors` : les producteurs présents à une distribution, dédupliqués, avec
  leur localité. L'accueil annonçait « qui sera là » sans pouvoir le dire.
- `GET /api/my-orders` : l'historique groupé par distribution, avec le cumul de
  l'année. L'endpoint existant exigeait une distribution précise, ce qui rendait
  tout historique impossible sans interroger chaque jeudi un par un.

Au passage, l'API et la page `/home` montraient **des périodes différentes** au
même adhérent : la première était restée à quatorze jours quand la seconde est
passée à trois semaines. Les deux partagent maintenant `homePeriodDays`.

### Tests

Vitest est entré dans le frontend avec ce chantier (`npm test`). Le calcul du
délai avant clôture est couvert, y compris le cas d'une date dépassée : un
« 0 jour » annoncerait des commandes ouvertes alors qu'elles sont closes.

## Ce qui reste à trancher

- **La date de bascule** pour les adhérents, et la façon de les prévenir. Ce
  n'est pas une décision technique.
- Le sort des templates Go une fois un écran migré : les retirer, ou les garder
  en secours quelques semaines.
