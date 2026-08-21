/**
 * Confirmation de commande.
 *
 * L'écran répond aux quatre questions qu'on se pose après avoir validé : ce
 * qu'on emporte, quand, où, et jusqu'à quand on peut encore changer d'avis. Le
 * panneau actuel de la boutique dit « Commande enregistrée ! » et s'arrête là.
 *
 * Le délai de modification est mis en avant plutôt qu'en note de bas de page :
 * c'est ce qui distingue une commande d'AMAP d'un achat en ligne, et c'est la
 * première chose qu'un adhérent cherche en revenant sur cet écran.
 */

export interface LigneCommande {
  produit: string
  quantite: string
  total: number
}

interface Props {
  jourLabel: string
  heureDebut: string
  heureFin: string
  lieu: string
  lignes: LigneCommande[]
  total: number
  /** « mardi 25 août à 20:00 », tel que le serveur le met en forme. */
  modifiableJusquA?: string
  onModifier: () => void
  onRetour: () => void
}

export function ConfirmationRefonte({
  jourLabel,
  heureDebut,
  heureFin,
  lieu,
  lignes,
  total,
  modifiableJusquA,
  onModifier,
  onRetour,
}: Props) {
  return (
    <div className="flex min-h-screen flex-col bg-surface text-ink">
      <header className="flex flex-col items-center gap-5 px-8 pt-16 text-center">
        <svg viewBox="0 0 76 76" className="size-[76px] fill-none" aria-hidden="true">
          <circle cx="38" cy="38" r="35" className="stroke-control" strokeWidth={2.5} />
          <path d="M23 39l10 11 20-24" className="stroke-control" strokeWidth={3.5} strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        <h1 className="m-0 font-display text-[34px] leading-tight">C'est noté, merci !</h1>
        <p className="m-0 text-base leading-relaxed text-control">
          Votre panier vous attendra
          <br />
          {jourLabel}, de {heureDebut} à {heureFin},
          <br />
          {lieu}.
        </p>
      </header>

      <section className="shadow-posee mx-6 mt-8 flex flex-col gap-3.5 rounded-card bg-card p-5">
        <div className="flex items-center justify-between">
          <h2 className="m-0 font-display text-lg italic">Votre panier</h2>
          <span className="font-display text-[22px]">{euros(total)}</span>
        </div>

        <span className="h-px bg-line" />

        <ul className="m-0 flex list-none flex-col gap-2 p-0 text-[15px]">
          {lignes.map((ligne, i) => (
            <li key={i} className="flex justify-between gap-4">
              <span>
                {ligne.quantite} {ligne.produit}
              </span>
              <span className="shrink-0 text-ink-muted">{euros(ligne.total)}</span>
            </li>
          ))}
        </ul>

        <p className="m-0 flex items-center gap-2 rounded-control bg-action-soft px-3.5 py-2.5 text-sm text-action-ink">
          <svg viewBox="0 0 24 24" className="size-4 shrink-0 fill-none stroke-action" strokeWidth={2} strokeLinecap="round">
            <path d="M12 20h9" />
            <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z" />
          </svg>
          {modifiableJusquA
            ? `Modifiable jusqu'au ${modifiableJusquA}.`
            : 'Les commandes sont closes : cette commande ne peut plus être modifiée.'}
        </p>
      </section>

      <div className="mt-7 flex flex-col items-center gap-3 px-6">
        {modifiableJusquA && (
          <button
            type="button"
            onClick={onModifier}
            className="min-h-[52px] rounded-control bg-control px-9 text-[17px] font-semibold text-card"
          >
            Modifier ma commande
          </button>
        )}
        <button
          type="button"
          onClick={onRetour}
          className="min-h-[52px] rounded-control border-[1.5px] border-ink/45 bg-transparent px-9 text-[17px] text-ink"
        >
          Retour à l'accueil
        </button>
      </div>
    </div>
  )
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}
