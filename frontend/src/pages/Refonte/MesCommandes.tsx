/**
 * Historique des commandes du compte.
 *
 * Regroupé par distribution et non par produit : un adhérent se souvient d'un
 * jeudi, pas d'une ligne de catalogue. La commande à venir est détachée en
 * tête, parce que c'est la seule sur laquelle il peut encore agir.
 */
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { fetchMyOrders, type MyOrder } from '../../api/myOrders'

export function MesCommandes() {
  const navigate = useNavigate()
  const { data, isLoading } = useQuery({ queryKey: ['my-orders'], queryFn: fetchMyOrders })

  if (isLoading || !data) {
    return <div className="flex min-h-screen items-center justify-center bg-canvas text-ink-muted">Chargement…</div>
  }

  const aVenir = data.orders.filter((o) => !o.past)
  const passees = data.orders.filter((o) => o.past)

  return (
    <div className="flex min-h-screen flex-col bg-canvas text-ink">
      <header className="bg-surface px-5 py-5">
        <h1 className="font-display text-[28px]">Mes commandes</h1>
        {data.nbOrders > 0 && (
          <p className="mt-1.5 text-[15px] text-control">
            {data.nbOrders} depuis janvier · {euros(data.totalYear)} au total
          </p>
        )}
      </header>

      <div className="flex flex-col gap-3.5 px-5 py-5">
        {data.orders.length === 0 && (
          <p className="text-center text-ink-muted">Vous n'avez pas encore commandé.</p>
        )}

        {aVenir.map((o) => (
          <button
            key={o.multiDistribId}
            type="button"
            onClick={() => navigate(`/shop/${o.multiDistribId}`)}
            className="shadow-posee flex items-center gap-3.5 rounded-card border-[1.5px] border-ink bg-card p-4 text-left"
          >
            <Date jour={o.day} mois={o.month} />
            <span className="h-11 w-px bg-line" />
            <span className="flex grow flex-col gap-0.5">
              <span className="text-[13px] font-semibold text-action-ink">À venir · modifiable</span>
              <span className="text-base">{o.nbArticles} article{o.nbArticles > 1 ? 's' : ''}</span>
            </span>
            <span className="font-display text-xl">{euros(o.total)}</span>
          </button>
        ))}

        {passees.length > 0 && (
          <div className="mt-1 flex items-center gap-3">
            <span className="font-display text-[17px] italic text-ink-muted">Retirées</span>
            <span className="h-px grow bg-line" />
          </div>
        )}

        <ul className="m-0 flex list-none flex-col p-0">
          {passees.map((o) => (
            <li key={o.multiDistribId} className="flex items-center gap-3.5 border-b border-line py-3.5">
              <span className="min-w-[34px] text-center leading-none">
                <span className="block font-display text-xl">{o.day}</span>
                <span className="block text-[11px] text-ink-muted">{o.month.slice(0, 4)}</span>
              </span>
              <span className="grow text-[15px]">{o.nbArticles} article{o.nbArticles > 1 ? 's' : ''}</span>
              <span className="font-display text-[17px]">{euros(o.total)}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}

function Date({ jour, mois }: { jour: number; mois: string }) {
  return (
    <span className="min-w-[46px] text-center leading-none">
      <span className="block font-display text-[28px]">{jour}</span>
      <span className="block text-xs text-ink-muted">{mois.slice(0, 4)}</span>
    </span>
  )
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}

export type { MyOrder }
