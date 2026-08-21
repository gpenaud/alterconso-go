/**
 * Choix de la distribution — deuxième étape du parcours.
 *
 * L'écran n'apparaît que si plusieurs distributions sont proposées : quand il
 * n'y en a qu'une, la poser en question serait une étape pour rien. C'est
 * l'accueil qui décide de l'afficher ou de conduire directement à la boutique.
 *
 * Les distributions passées restent visibles, en retrait : un adhérent y
 * cherche ce qu'il a commandé la dernière fois.
 */

export interface DistributionListee {
  id: number
  jourSemaine: string
  jour: number
  mois: string
  heureDebut: string
  heureFin: string
  lieu: string
  /** Mis en forme par l'appelant : « encore 2 jours », « ouvre le 3 septembre ». */
  etat: string
  ouverte: boolean
  passee: boolean
  /** Montant déjà commandé, s'il y en a un. */
  montantCommande?: number
}

interface Props {
  distributions: DistributionListee[]
  onChoisir: (id: number) => void
  onRetour: () => void
}

export function DistributionsRefonte({ distributions, onChoisir, onRetour }: Props) {
  const aVenir = distributions.filter((d) => !d.passee)
  const passees = distributions.filter((d) => d.passee)

  return (
    <div className="flex min-h-screen flex-col bg-canvas text-ink">
      <header className="flex items-center gap-3 bg-surface px-5 py-4">
        <button type="button" onClick={onRetour} aria-label="Revenir" className="bg-transparent p-0">
          <svg viewBox="0 0 24 24" className="size-[22px] fill-none stroke-ink" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
            <path d="M15 6l-6 6 6 6" />
          </svg>
        </button>
        <h1 className="font-display text-xl">Choisir une distribution</h1>
      </header>

      <ul className="flex list-none flex-col gap-3 p-4">
        {aVenir.map((d) => (
          <li key={d.id}>
            <button
              type="button"
              onClick={() => onChoisir(d.id)}
              className={`flex w-full items-center gap-4 rounded-card border-[1.5px] bg-card p-4 text-left ${
                d.ouverte ? 'border-control' : 'border-line'
              }`}
            >
              <Date jourSemaine={d.jourSemaine} jour={d.jour} mois={d.mois} accentue={d.ouverte} />
              <span className="flex grow flex-col gap-1">
                <span className="text-base font-semibold">{d.heureDebut} – {d.heureFin}</span>
                <span className="text-sm text-ink-muted">{d.lieu}</span>
                <span className={`text-sm ${d.ouverte ? 'text-action-ink' : 'text-ink-muted'}`}>{d.etat}</span>
                {d.montantCommande != null && (
                  <span className="text-sm text-control">Commande en cours · {euros(d.montantCommande)}</span>
                )}
              </span>
              <Chevron actif={d.ouverte} />
            </button>
          </li>
        ))}

        {passees.length > 0 && (
          <li className="mt-2 flex items-center gap-3">
            <span className="font-display text-[17px] italic text-ink-muted">Passées</span>
            <span className="h-px grow bg-line" />
          </li>
        )}

        {passees.map((d) => (
          <li key={d.id}>
            <button
              type="button"
              onClick={() => onChoisir(d.id)}
              className="flex w-full items-center gap-4 rounded-card border-[1.5px] border-line bg-card p-4 text-left opacity-60"
            >
              <Date jourSemaine={d.jourSemaine} jour={d.jour} mois={d.mois} accentue={false} />
              <span className="flex grow flex-col gap-1">
                <span className="text-base">{d.etat}</span>
                {d.montantCommande != null && (
                  <span className="text-sm text-ink-muted">{euros(d.montantCommande)}</span>
                )}
              </span>
              <Chevron actif={false} />
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}

function Date({ jourSemaine, jour, mois, accentue }: { jourSemaine: string; jour: number; mois: string; accentue: boolean }) {
  return (
    <span className={`flex min-w-[54px] flex-col items-center leading-none ${accentue ? 'text-control' : 'text-ink'}`}>
      <span className="text-[13px]">{jourSemaine}</span>
      <span className="my-0.5 font-display text-[30px]">{jour}</span>
      <span className="text-[13px]">{mois}</span>
    </span>
  )
}

function Chevron({ actif }: { actif: boolean }) {
  return (
    <svg viewBox="0 0 24 24" className={`size-5 shrink-0 fill-none ${actif ? 'stroke-control' : 'stroke-ink-faint'}`} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 6l6 6-6 6" />
    </svg>
  )
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}
