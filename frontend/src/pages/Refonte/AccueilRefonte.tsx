/**
 * Écran d'accueil du parcours repensé — référence de la refonte.
 *
 * Il ne pose qu'une question : « je commande pour la prochaine distribution ».
 * Tout le reste — historique, producteurs, compte — attend derrière la barre du
 * bas. C'est ce resserrement qui distingue ce parcours de l'accueil actuel, où
 * l'adhérent choisit d'abord un groupe, puis une distribution, puis un
 * catalogue.
 *
 * Composant de présentation : il reçoit ses données, il n'en cherche aucune.
 * Le branchement sur l'API se fera dans la page qui l'enveloppe, une fois la
 * direction validée sur écran.
 */

export interface Producteur {
  id: number
  nom: string
  /** Localité : elle dit la proximité, qui est ce qui distingue une AMAP d'une épicerie. */
  ville?: string
  bio?: boolean
  /** Dessin au trait, en attendant que les producteurs fournissent des photos. */
  embleme: 'legume' | 'oeuf' | 'fromage'
}

export interface ProchaineDistribution {
  jourSemaine: string
  jour: number
  mois: string
  heureDebut: string
  heureFin: string
  lieu: string
  /** Déjà mis en forme : « 2 jours », « 4 heures ». La formulation en mots
   *  plutôt qu'un compte à rebours évite de faire clignoter l'urgence. */
  tempsRestant: string
  nbProducteurs: number
  nbProduits: number
}

interface Props {
  prenom: string
  groupe: string
  distribution: ProchaineDistribution
  producteurs: Producteur[]
  onCommander: () => void
}

export function AccueilRefonte({ prenom, groupe, distribution, producteurs, onCommander }: Props) {
  return (
    <div className="flex min-h-screen flex-col bg-canvas text-ink">
      <header className="relative overflow-hidden bg-surface px-6 pb-32 pt-5">
        <MotifChamps />

        <div className="relative flex items-center justify-between">
          <span className="text-xs uppercase tracking-[0.18em] text-surface-deep">{groupe}</span>
          <div className="size-9 rounded-full border-[1.5px] border-surface-deep/45 bg-surface-deep/10" />
        </div>

        <div className="relative mt-6">
          <p className="font-display text-xl italic">Bonjour {prenom},</p>
          <p className="mt-2.5 text-[15px] text-control">on se retrouve</p>
          <div className="mt-0.5 flex items-baseline gap-3.5">
            <span className="font-display text-[88px] leading-[0.85]">{distribution.jour}</span>
            <span className="flex flex-col">
              <span className="font-display text-[27px] italic">{distribution.mois}</span>
              <span className="mt-1 text-[15px] text-control">{distribution.jourSemaine}</span>
            </span>
          </div>
        </div>
      </header>

      <section className="shadow-posee relative -mt-24 mx-5 flex flex-col gap-4 rounded-card border-[1.5px] border-ink bg-card p-[22px]">
        <div className="flex flex-col gap-2.5">
          <Ligne icone={<IconeHorloge />}>{distribution.heureDebut} – {distribution.heureFin}</Ligne>
          <Ligne icone={<IconeLieu />}>{distribution.lieu}</Ligne>
        </div>

        <p className="flex items-center gap-2.5 rounded-control bg-action-soft px-3.5 py-2.5 text-[15px] text-action-ink">
          <IconeHorloge className="stroke-action" />
          Encore <strong>{distribution.tempsRestant}</strong> pour commander
        </p>

        <button
          type="button"
          onClick={onCommander}
          className="flex min-h-[56px] items-center justify-center gap-3 self-center rounded-control bg-action px-9 text-lg font-semibold text-card"
        >
          Composer mon panier
          <IconeFleche />
        </button>
      </section>

      {producteurs.length > 0 && (
      <section className="mt-6 flex flex-col gap-3.5 px-5">
        <div className="flex items-center gap-3">
          <h2 className="font-display text-lg italic">Qui sera là</h2>
          <span className="h-px grow bg-line" />
          <span className="text-sm text-ink-muted">{distribution.nbProducteurs} producteurs</span>
        </div>

        <ul className="flex list-none gap-3 p-0">
          {producteurs.map((producteur) => (
            <li
              key={producteur.id}
              className="flex grow flex-col items-center gap-2.5 rounded-card border-[1.5px] border-line bg-card px-3 py-3.5"
            >
              <Embleme type={producteur.embleme} />
              <span className="text-center text-[13px] leading-tight">{producteur.nom}</span>
              {producteur.ville && (
                <span className="text-center text-[11px] leading-tight text-ink-muted">{producteur.ville}</span>
              )}
              {producteur.bio && (
                // Mention discrète : elle informe sans transformer la rangée en
                // mur d'étiquettes — ici, presque tout le monde est bio.
                <span className="rounded-full bg-tint px-2 text-[10px] uppercase tracking-wider text-control">
                  bio
                </span>
              )}
            </li>
          ))}
        </ul>
      </section>
      )}
    </div>
  )
}

function Ligne({ icone, children }: { icone: React.ReactNode; children: React.ReactNode }) {
  return (
    <p className="flex items-center gap-3 text-[17px]">
      {icone}
      {children}
    </p>
  )
}

/* Les emblèmes tiennent lieu de photos tant que les catalogues n'en ont pas.
 * Un dessin assumé vaut mieux qu'une vignette grise ou qu'une photo générique
 * qui ne serait pas celle du producteur. */
function Embleme({ type }: { type: Producteur['embleme'] }) {
  const traits = {
    legume: (
      <>
        <path d="M8 30c0-9 5-15 12-15s12 6 12 15" />
        <path d="M20 15c-3-8 1-14 7-16 1 7-2 13-7 16z" />
      </>
    ),
    oeuf: (
      <>
        <ellipse cx="20" cy="24" rx="11" ry="13" />
        <path d="M20 11c4-4 10-3 12 0-3 4-9 4-12 0z" />
      </>
    ),
    fromage: (
      <>
        <path d="M8 26l12-14 12 14z" />
        <path d="M8 26h24v6H8z" />
      </>
    ),
  }[type]

  const couleur = { legume: 'stroke-control', oeuf: 'stroke-action', fromage: 'stroke-ink-muted' }[type]

  return (
    <svg viewBox="0 0 40 40" className={`size-[34px] fill-none ${couleur}`} strokeWidth={2} strokeLinecap="round">
      {traits}
    </svg>
  )
}

function IconeHorloge({ className = 'stroke-control' }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={`size-[18px] shrink-0 fill-none ${className}`} strokeWidth={2} strokeLinecap="round">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </svg>
  )
}

function IconeLieu() {
  return (
    <svg viewBox="0 0 24 24" className="size-[18px] shrink-0 fill-none stroke-control" strokeWidth={2} strokeLinecap="round">
      <path d="M12 21s7-5.6 7-11a7 7 0 1 0-14 0c0 5.4 7 11 7 11z" />
      <circle cx="12" cy="10" r="2.5" />
    </svg>
  )
}

function IconeFleche() {
  return (
    <svg viewBox="0 0 24 24" className="size-5 fill-none stroke-card" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 12h13" />
      <path d="M13 6l6 6-6 6" />
    </svg>
  )
}

/* Décor de champs, en filigrane derrière le bandeau. Purement ornemental :
 * masqué aux lecteurs d'écran. */
function MotifChamps() {
  return (
    <svg
      viewBox="0 0 390 240"
      aria-hidden="true"
      className="absolute inset-0 size-full fill-none stroke-surface-deep opacity-20"
      strokeWidth={2}
      strokeLinecap="round"
    >
      <path d="M-10 176c26-4 40-20 44-44 22 10 42 4 54-14 16 18 38 22 58 10 6 26 24 42 50 44" />
      <path d="M150 60c-4-18 4-32 18-38 2 16-4 30-18 38z" />
      <path d="M150 60c-14-12-30-12-40-4 10 12 26 14 40 4z" />
      <circle cx="300" cy="70" r="26" />
      <path d="M300 44c6-10 18-12 24-8-3 10-13 15-24 8z" />
      <path d="M40 90c12-6 26-4 34 4-10 8-26 8-34-4z" />
    </svg>
  )
}
