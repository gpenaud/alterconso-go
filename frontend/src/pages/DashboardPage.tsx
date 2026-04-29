import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { getGroup } from '../api/groups'
import { getBalance, getOperations } from '../api/finance'
import { fetchHome } from '../api/home'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'
import { MultiDistribCard } from './Dashboard/MultiDistribCard'

const opTypeLabel: Record<string, string> = {
  VOrder: 'Commande variable',
  COrder: 'Commande AMAP',
  Payment: 'Paiement',
  Membership: 'Adhésion',
}

export function DashboardPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const id = Number(groupId)
  const [offset, setOffset] = useState(0)

  const { data: group } = useQuery({ queryKey: ['group', id], queryFn: () => getGroup(id) })
  const { data: home, isLoading: homeLoading } = useQuery({
    queryKey: ['home', id, offset],
    queryFn: () => fetchHome(offset),
  })
  const { data: balance } = useQuery({ queryKey: ['balance', id], queryFn: () => getBalance(id) })
  const { data: operations = [] } = useQuery({
    queryKey: ['operations', id],
    queryFn: () => getOperations(id),
  })

  const balanceColor = balance === undefined ? 'text-gray-500'
    : balance >= 0 ? 'text-ac-green-dark' : 'text-red-600'

  return (
    <Layout title={group?.name ?? '…'} backTo="/groups" backLabel="Groupes">
      <div className="space-y-6">
        {/* Solde — résumé compact */}
        <Card>
          <div className="px-6 py-4 flex items-center justify-between flex-wrap gap-4">
            <div>
              <p className="text-sm text-gray-500">Mon solde</p>
              <p className={`text-2xl font-bold mt-0.5 ${balanceColor}`}>
                {balance !== undefined ? `${balance >= 0 ? '+' : ''}${balance.toFixed(2)} €` : '—'}
              </p>
            </div>
          </div>
        </Card>

        {/* Section home : 2 colonnes (distribs à gauche, infos groupe à droite) */}
        <section className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div className="md:col-span-2 space-y-4">
            {homeLoading ? (
              <p className="text-sm text-gray-500 py-8 text-center">Chargement…</p>
            ) : home && home.multiDistribs.length > 0 ? (
              home.multiDistribs.map((md) => (
                <MultiDistribCard key={md.id} md={md} />
              ))
            ) : (
              <p className="text-sm text-gray-500 py-8 text-center">
                Il n'y a pas de distribution prévue pour le moment.
              </p>
            )}

            {/* Navigation période */}
            {home && (
              <div className="flex items-center justify-center gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setOffset((v) => v - 1)}
                  className="px-3 py-1.5 rounded border border-gray-300 text-gray-600 hover:bg-gray-50"
                  aria-label="Période précédente"
                >
                  <i className="icon-chevron-left" aria-hidden="true" />
                </button>
                <span className="px-3 py-1.5 text-sm text-gray-600 select-none">
                  {home.periodLabel}
                </span>
                <button
                  type="button"
                  onClick={() => setOffset((v) => v + 1)}
                  className="px-3 py-1.5 rounded border border-gray-300 text-gray-600 hover:bg-gray-50"
                  aria-label="Période suivante"
                >
                  <i className="icon-chevron-right" aria-hidden="true" />
                </button>
              </div>
            )}
          </div>

          {/* Colonne droite : texte d'accueil du groupe + raccourcis */}
          <aside className="space-y-4">
            {home?.groupTxtHome && (
              <Card>
                <div
                  className="px-5 py-4 text-sm text-gray-700 whitespace-pre-line"
                >
                  {home.groupTxtHome}
                </div>
              </Card>
            )}
            <a
              href="/distribution/volunteersCalendar"
              className="inline-flex items-center gap-2 px-3 py-2 rounded-md text-sm border border-gray-300 text-gray-700 bg-white hover:bg-gray-50"
            >
              <i className="icon-calendar" aria-hidden="true" />
              Calendrier des permanences
            </a>
          </aside>
        </section>

        {/* Dernières opérations */}
        <Card>
          <CardHeader
            title="Dernières opérations"
            subtitle="10 dernières"
          />
          <div className="divide-y divide-gray-100">
            {operations.length === 0 ? (
              <p className="px-6 py-4 text-sm text-gray-500">Aucune opération.</p>
            ) : (
              operations.slice(0, 10).map((op) => (
                <div key={op.id} className="px-6 py-3 flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">
                      {op.description ?? opTypeLabel[op.type] ?? op.type}
                    </p>
                    <p className="text-xs text-gray-400 mt-0.5">
                      {new Date(op.date).toLocaleDateString('fr-FR')}
                      {op.pending && <span className="ml-2 text-orange-500">en attente</span>}
                    </p>
                  </div>
                  <span className={`text-sm font-semibold ${op.amount >= 0 ? 'text-ac-green-dark' : 'text-red-600'}`}>
                    {op.amount >= 0 ? '+' : ''}{op.amount.toFixed(2)} €
                  </span>
                </div>
              ))
            )}
          </div>
        </Card>
      </div>
    </Layout>
  )
}
