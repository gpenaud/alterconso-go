/**
 * Les commandes d'une distribution, sous deux angles.
 *
 * Par produit pour préparer avec les producteurs, par adhérent pour distribuer.
 * Ce sont deux moments différents du même jeudi, et l'écran bascule de l'un à
 * l'autre sans recharger : les lignes sont les mêmes, seul le regroupement
 * change.
 */
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchHome } from '../../api/home'
import { fetchDistributionOrders, type AdminOrderLine } from '../../api/adminOrders'

type Angle = 'produit' | 'adherent'

export function AdminCommandes() {
  const [angle, setAngle] = useState<Angle>('produit')
  const { data: accueil } = useQuery({ queryKey: ['home', 0], queryFn: () => fetchHome(0) })
  const distribution = accueil?.multiDistribs.find((d) => d.canOrder) ?? accueil?.multiDistribs.find((d) => !d.past)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-orders', distribution?.id],
    queryFn: () => fetchDistributionOrders(distribution!.id),
    enabled: Boolean(distribution),
  })

  const groupes = useMemo(() => regrouper(data?.lines ?? [], angle), [data, angle])

  if (!distribution) return <Message>Aucune distribution n'est programmée.</Message>
  if (isLoading) return <Message>Chargement…</Message>

  return (
    <div className="flex flex-col">
      <header className="flex items-center justify-between gap-6 bg-surface px-8 py-4">
        <div className="flex flex-col gap-1">
          <h1 className="m-0 font-display text-2xl">
            Commandes · {distribution.dayOfWeek} {distribution.day} {distribution.month}
          </h1>
          <p className="m-0 text-sm text-surface-deep">
            {(data?.lines.length ?? 0)} lignes · {euros(data?.total ?? 0)}
          </p>
        </div>
        <a
          href={`/contractAdmin/ordersByDate/${distribution.id}`}
          className="min-h-[44px] rounded-control border-[1.5px] border-surface-deep bg-card px-5 text-[15px] leading-[44px] text-ink no-underline"
        >
          Vue détaillée
        </a>
      </header>

      <div className="flex items-center gap-3 px-8 pt-5">
        <div className="flex gap-1 rounded-control bg-tint p-1">
          {(['produit', 'adherent'] as const).map((valeur) => (
            <button
              key={valeur}
              type="button"
              onClick={() => setAngle(valeur)}
              className={`min-h-11 rounded-[3px] px-5 text-[15px] ${
                angle === valeur ? 'bg-card font-semibold' : 'bg-transparent text-ink-muted'
              }`}
            >
              {valeur === 'produit' ? 'Par produit' : 'Par adhérent'}
            </button>
          ))}
        </div>
      </div>

      <div className="p-8 pt-4">
        <div className="overflow-hidden rounded-card border-[1.5px] border-line bg-card">
          {groupes.map((groupe) => (
            <div key={groupe.titre}>
              <h2 className="m-0 border-b border-line bg-tint/40 px-5 py-3 font-display text-[17px] italic">
                {groupe.titre}
                <span className="ml-2 font-body text-sm not-italic text-ink-muted">
                  {groupe.lignes.length} ligne{groupe.lignes.length > 1 ? 's' : ''} · {euros(groupe.total)}
                </span>
              </h2>

              {groupe.lignes.map((ligne, i) => (
                <div key={i} className="grid grid-cols-[90px_1fr_80px_110px_120px] items-center gap-3 border-b border-line px-5 py-3 last:border-b-0">
                  <span className="text-sm text-ink-muted">{ligne.productRef || '—'}</span>
                  <span className="text-base">{angle === 'produit' ? ligne.userName : ligne.product}</span>
                  <span className="text-right font-display text-[17px]">{arrondi(ligne.quantity)}</span>
                  <span className="text-right text-[15px]">
                    {ligne.needsWeighing ? (
                      <span className={`rounded-[3px] px-2 py-1 text-sm ${ligne.weighed ? 'bg-tint text-ink-muted' : 'bg-action-soft text-action-ink'}`}>
                        {ligne.weighed ? 'pesé' : 'à peser'}
                      </span>
                    ) : (
                      euros(ligne.unitPrice)
                    )}
                  </span>
                  <span className="text-right font-display text-[17px]">{euros(ligne.total)}</span>
                </div>
              ))}
            </div>
          ))}

          {groupes.length === 0 && <p className="m-0 px-5 py-6 text-ink-muted">Aucune commande pour l'instant.</p>}

          <div className="grid grid-cols-[90px_1fr_80px_110px_120px] items-center gap-3 bg-surface-deep px-5 py-4 text-card">
            <span />
            <span className="font-display text-lg italic">Total général</span>
            <span />
            <span />
            <span className="text-right font-display text-xl">{euros(data?.total ?? 0)}</span>
          </div>
        </div>
      </div>
    </div>
  )
}

interface Groupe {
  titre: string
  lignes: AdminOrderLine[]
  total: number
}

/** Le regroupement est ici, et non côté serveur : les mêmes lignes servent aux
 *  deux vues, et basculer ne doit pas coûter une requête. */
function regrouper(lignes: AdminOrderLine[], angle: Angle): Groupe[] {
  const parClef = new Map<string, Groupe>()
  for (const ligne of lignes) {
    const titre = angle === 'produit' ? ligne.product : ligne.userName
    const groupe = parClef.get(titre) ?? { titre, lignes: [], total: 0 }
    groupe.lignes.push(ligne)
    groupe.total += ligne.total
    parClef.set(titre, groupe)
  }
  return [...parClef.values()].sort((a, b) => a.titre.localeCompare(b.titre, 'fr'))
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center px-8 text-center text-ink-muted">{children}</div>
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}

function arrondi(quantite: number) {
  return Number.isInteger(quantite) ? String(quantite) : quantite.toFixed(1)
}
