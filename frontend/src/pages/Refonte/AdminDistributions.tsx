/**
 * Calendrier des distributions.
 *
 * La prochaine est détachée, les suivantes en lignes plus calmes, les passées
 * en retrait : un responsable prépare l'une, surveille les autres, et ne
 * consulte les dernières que pour vérifier un chiffre.
 */
import { useQuery } from '@tanstack/react-query'
import { fetchAdminDistributions, type AdminDistribution } from '../../api/adminDistributions'
import { tempsRestant } from './tempsRestant'

export function AdminDistributions() {
  const { data, isLoading } = useQuery({
    queryKey: ['admin-distributions'],
    queryFn: fetchAdminDistributions,
  })

  if (isLoading || !data) return <Message>Chargement…</Message>

  const aVenir = data.filter((d) => !d.past).sort((a, b) => a.startAt.localeCompare(b.startAt))
  const passees = data.filter((d) => d.past)
  const [prochaine, ...suivantes] = aVenir

  return (
    <div className="flex flex-col">
      <header className="flex items-center justify-between gap-6 bg-surface px-8 py-4">
        <div className="flex flex-col gap-1">
          <h1 className="m-0 font-display text-2xl">Distributions</h1>
          <p className="m-0 text-sm text-surface-deep">{aVenir.length} à venir · {passees.length} passées</p>
        </div>
        <a
          href="/distribution/insertMd"
          className="min-h-[44px] rounded-control bg-action px-5 text-[15px] font-semibold leading-[44px] text-card no-underline"
        >
          Ajouter une date
        </a>
      </header>

      <div className="flex flex-col gap-3 p-8">
        {prochaine && <Carte distribution={prochaine} mise />}
        {suivantes.map((d) => (
          <Carte key={d.id} distribution={d} />
        ))}

        {passees.length > 0 && (
          <div className="mt-3 flex items-center gap-3">
            <span className="font-display text-[17px] italic text-ink-muted">Passées</span>
            <span className="h-px grow bg-line" />
          </div>
        )}

        {passees.map((d) => (
          <div key={d.id} className="flex items-center gap-5 px-5 py-3 opacity-70">
            <Date jour={d.day} mois={d.month} />
            <span className="grow text-[15px]">
              {d.nbOrders} commande{d.nbOrders > 1 ? 's' : ''} · {euros(d.total)}
            </span>
            <a href={`/distribution/list/${d.id}`} className="text-sm text-control no-underline">
              Voir
            </a>
          </div>
        ))}
      </div>
    </div>
  )
}

function Carte({ distribution: d, mise }: { distribution: AdminDistribution; mise?: boolean }) {
  const delai = tempsRestant(d.orderEndAt)

  return (
    <article
      className={`flex items-center gap-5 rounded-card bg-card p-5 ${
        mise ? 'shadow-posee border-[1.5px] border-ink' : 'border-[1.5px] border-line'
      }`}
    >
      <Date jour={d.day} mois={d.month} grand={mise} />
      <span className="h-14 w-px bg-line" />

      <div className="flex grow flex-col gap-1.5">
        <div className="flex items-center gap-2.5">
          <span className={`size-2 rounded-full ${d.open ? 'bg-control' : 'bg-ink-faint'}`} />
          {d.open ? (
            <span className="text-base font-semibold text-control">
              Commandes ouvertes{delai && ` · encore ${delai}`}
            </span>
          ) : (
            <span className="text-base text-ink-muted">
              {d.orderStartLabel ? `Ouvre le ${d.orderStartLabel}` : 'Commandes fermées'}
            </span>
          )}
        </div>

        <p className="m-0 text-[15px]">
          {d.startHour} – {d.endHour} · {d.place} · {d.nbVendors} producteur{d.nbVendors > 1 ? 's' : ''}
          {d.nbOrders > 0 && ` · ${d.nbOrders} commande${d.nbOrders > 1 ? 's' : ''} · ${euros(d.total)}`}
        </p>

        {d.volunteerNeeded > 0 && (
          <p className="m-0 text-sm text-action-ink">
            {d.volunteerNeeded} bénévole{d.volunteerNeeded > 1 ? 's' : ''} manquant
            {d.volunteerNeeded > 1 ? 's' : ''}
          </p>
        )}
      </div>

      <nav className="flex shrink-0 gap-3 text-sm">
        <a href={`/distribution/inviteFarmers/${d.id}`} className="text-control no-underline">Producteurs</a>
        <span className="text-ink/25">·</span>
        <a href={`/distribution/volunteersSummary/${d.id}`} className="text-control no-underline">Permanences</a>
        <span className="text-ink/25">·</span>
        <a href={`/distribution/editMd/${d.id}`} className="text-control no-underline">Modifier</a>
      </nav>
    </article>
  )
}

function Date({ jour, mois, grand }: { jour: number; mois: string; grand?: boolean }) {
  return (
    <span className="flex min-w-[58px] flex-col items-center leading-none">
      <span className={`font-display ${grand ? 'text-[34px]' : 'text-[30px]'}`}>{jour}</span>
      <span className="mt-1 text-[13px] text-ink-muted">{mois.slice(0, 4)}</span>
    </span>
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center px-8 text-center text-ink-muted">{children}</div>
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 })
}
