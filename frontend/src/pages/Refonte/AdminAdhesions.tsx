/**
 * Suivi des adhésions.
 *
 * L'écran s'ouvre sur les retards, pas sur les gens à jour : c'est la liste
 * qu'on vient chercher. Et la colonne des commandes de l'année les départage —
 * celui qui a oublié de payer mais commande chaque semaine n'est pas celui qui
 * a quitté le groupe sans le dire.
 */
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchMemberships } from '../../api/adminMemberships'

export function AdminAdhesions() {
  const [onglet, setOnglet] = useState<'retard' | 'ajour'>('retard')
  const { data, isLoading, isError } = useQuery({ queryKey: ['memberships'], queryFn: fetchMemberships })

  if (isLoading) return <Message>Chargement…</Message>
  if (isError || !data) return <Message>Le suivi des adhésions ne vous est pas accessible.</Message>

  if (!data.hasMembership) {
    return <Message>Ce groupe ne gère pas d'adhésion annuelle.</Message>
  }

  const retards = data.members.filter((m) => !m.upToDate)
  const aJour = data.members.filter((m) => m.upToDate)
  const affiches = onglet === 'retard' ? retards : aJour

  return (
    <div className="flex flex-col">
      <header className="flex items-center justify-between gap-6 bg-surface px-8 py-4">
        <h1 className="m-0 font-display text-2xl">Adhésions {data.year}</h1>
        {retards.length > 0 && (
          <a
            href="/messages"
            className="min-h-[44px] rounded-control bg-action px-5 text-[15px] font-semibold leading-[44px] text-card no-underline"
          >
            Relancer les {retards.length} retards
          </a>
        )}
      </header>

      <div className="flex items-start gap-6 p-8">
        <aside className="flex w-[340px] shrink-0 flex-col gap-4">
          <section className="overflow-hidden rounded-card border-[1.5px] border-line bg-card">
            <h2 className="m-0 border-b border-line px-[18px] py-3.5 font-display text-[17px] italic">Réglages</h2>
            <dl className="m-0 flex flex-col gap-3.5 p-[18px]">
              <div>
                <dt className="text-xs uppercase tracking-wider text-ink-muted">Montant</dt>
                <dd className="m-0 mt-1 font-display text-lg">{euros(data.fee)}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-ink-muted">Renouvellement</dt>
                <dd className="m-0 mt-1 text-base">{data.renewalDate || 'Non défini'}</dd>
              </div>
            </dl>
            <p className="m-0 border-t border-line px-[18px] py-3.5 text-sm leading-relaxed text-ink-muted">
              Un adhérent sans adhésion à jour peut commander ; il apparaît seulement dans cette liste.
            </p>
            <a
              href="/amapadmin/membership"
              className="block border-t border-line px-[18px] py-3.5 text-[15px] text-control no-underline"
            >
              Modifier les réglages
            </a>
          </section>

          <section className="flex flex-col gap-3.5 rounded-card border-[1.5px] border-line bg-card p-[18px]">
            <h2 className="m-0 font-display text-[17px] italic">Cette année</h2>
            <p className="m-0 flex items-baseline gap-2.5">
              <span className="font-display text-[38px] leading-none">{euros(data.collectedYear)}</span>
              <span className="text-sm text-ink-muted">collectés</span>
            </p>
            <span className="flex h-3 overflow-hidden rounded-full bg-tint">
              <span
                className="block h-full bg-control"
                style={{ width: `${pourcentage(data.upToDate, data.members.length)}%` }}
              />
            </span>
            <p className="m-0 flex justify-between text-sm">
              <span className="text-control">{data.upToDate} à jour</span>
              <span className="text-action-ink">{data.late} en retard</span>
            </p>
          </section>
        </aside>

        <div className="grow overflow-hidden rounded-card border-[1.5px] border-line bg-card">
          <div className="flex items-center gap-3 border-b border-line px-5 py-3">
            <div className="flex gap-1 rounded-control bg-tint p-1">
              <Onglet actif={onglet === 'retard'} onClick={() => setOnglet('retard')}>
                En retard · {retards.length}
              </Onglet>
              <Onglet actif={onglet === 'ajour'} onClick={() => setOnglet('ajour')}>
                À jour · {aJour.length}
              </Onglet>
            </div>
          </div>

          <div className="grid grid-cols-[1.6fr_1.4fr_140px_120px] gap-3 border-b border-line bg-tint/60 px-5 py-3 text-xs uppercase tracking-wider text-ink-muted">
            <span>Adhérent</span>
            <span>Dernière adhésion</span>
            <span>Commandes {data.year}</span>
            <span className="text-right">Action</span>
          </div>

          {affiches.map((membre) => (
            <div
              key={membre.userId}
              className="grid grid-cols-[1.6fr_1.4fr_140px_120px] items-center gap-3 border-b border-line px-5 py-3 last:border-b-0"
            >
              <span className="text-[15px]">{membre.name}</span>
              <span className={`text-[15px] ${membre.upToDate ? 'text-control' : 'text-action-ink'}`}>
                {etatAdhesion(membre.lastYear, data.year)}
              </span>
              <span className="text-[15px] text-ink-muted">
                {membre.nbOrdersThisYear > 0 ? `${membre.nbOrdersThisYear} commandes` : 'aucune'}
              </span>
              <a href={`/member/view/${membre.userId}`} className="text-right text-sm text-control no-underline">
                Ouvrir
              </a>
            </div>
          ))}

          {affiches.length === 0 && (
            <p className="m-0 px-5 py-6 text-ink-muted">
              {onglet === 'retard' ? 'Personne n’est en retard.' : 'Personne n’est à jour.'}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}

function Onglet({ actif, onClick, children }: { actif: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`min-h-11 rounded-[3px] px-4 text-[15px] ${actif ? 'bg-card font-semibold' : 'bg-transparent text-ink-muted'}`}
    >
      {children}
    </button>
  )
}

/** « Jamais adhéré » n'est pas un retard : la distinction change la relance. */
function etatAdhesion(derniereAnnee: number, anneeCourante: number) {
  if (derniereAnnee === 0) return 'Jamais adhéré'
  if (derniereAnnee >= anneeCourante) return `À jour · ${derniereAnnee}`
  const ecart = anneeCourante - derniereAnnee
  return `${derniereAnnee} · expirée depuis ${ecart} an${ecart > 1 ? 's' : ''}`
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center px-8 text-center text-ink-muted">{children}</div>
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}

function pourcentage(valeur: number, total: number) {
  return total > 0 ? Math.round((valeur / total) * 100) : 0
}
