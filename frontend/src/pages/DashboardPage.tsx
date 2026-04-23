import { useQuery } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { getGroup } from '../api/groups'
import { getBalance, getOperations } from '../api/finance'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'

const opTypeLabel: Record<string, string> = {
  VOrder: 'Commande variable',
  COrder: 'Commande AMAP',
  Payment: 'Paiement',
  Membership: 'Adhésion',
}

export function DashboardPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const id = Number(groupId)

  const { data: group } = useQuery({ queryKey: ['group', id], queryFn: () => getGroup(id) })
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
        {/* Solde + raccourcis */}
        <Card>
          <div className="px-6 py-5 flex items-center justify-between flex-wrap gap-4">
            <div>
              <p className="text-sm text-gray-500">Mon solde</p>
              <p className={`text-3xl font-bold mt-1 ${balanceColor}`}>
                {balance !== undefined ? `${balance >= 0 ? '+' : ''}${balance.toFixed(2)} €` : '—'}
              </p>
            </div>
            <Link
              to={`/groups/${id}/distributions`}
              className="inline-flex items-center px-4 py-2 rounded-md text-sm font-medium
                bg-ac-green hover:bg-ac-green-dark text-white transition-colors"
            >
              Commander
            </Link>
          </div>
        </Card>

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
