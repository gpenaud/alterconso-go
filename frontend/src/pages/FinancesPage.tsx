import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { getBalance, getOperations } from '../api/finance'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'

const opTypeLabel: Record<string, string> = {
  VOrder: 'Commande variable',
  COrder: 'Commande AMAP',
  Payment: 'Paiement',
  Membership: 'Adhésion',
}

const paymentTypeLabel: Record<string, string> = {
  cash: 'Espèces',
  check: 'Chèque',
  transfer: 'Virement',
  moneypot: 'Cagnotte',
  onthespot: 'Sur place',
  cardterminal: 'Carte (TPE)',
}

export function FinancesPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const id = Number(groupId)

  const { data: balance } = useQuery({ queryKey: ['balance', id], queryFn: () => getBalance(id) })
  const { data: operations = [], isLoading } = useQuery({
    queryKey: ['operations', id],
    queryFn: () => getOperations(id, 100),
  })

  const balanceColor = balance === undefined ? 'text-gray-400'
    : balance >= 0 ? 'text-ac-green-dark' : 'text-red-600'

  return (
    <Layout title="Mes finances">
      <div className="space-y-6">
        <Card>
          <div className="px-6 py-5">
            <p className="text-sm text-gray-500 mb-1">Solde actuel</p>
            <p className={`text-4xl font-bold ${balanceColor}`}>
              {balance !== undefined
                ? `${balance >= 0 ? '+' : ''}${balance.toFixed(2)} €`
                : '—'}
            </p>
            {balance !== undefined && balance < 0 && (
              <p className="text-sm text-red-500 mt-2">
                Vous avez une dette de {Math.abs(balance).toFixed(2)} €.
              </p>
            )}
          </div>
        </Card>

        <Card>
          <CardHeader title="Historique" subtitle={`${operations.length} opération(s)`} />
          {isLoading ? (
            <p className="px-6 py-4 text-sm text-gray-500">Chargement…</p>
          ) : operations.length === 0 ? (
            <p className="px-6 py-4 text-sm text-gray-500">Aucune opération.</p>
          ) : (
            <div className="divide-y divide-gray-100">
              {operations.map((op) => (
                <div key={op.id} className="px-6 py-3 flex items-center justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900 truncate">
                      {op.description ?? opTypeLabel[op.type] ?? op.type}
                    </p>
                    <p className="text-xs text-gray-400 mt-0.5 flex items-center gap-2">
                      <span>{new Date(op.date).toLocaleDateString('fr-FR')}</span>
                      {op.paymentType && (
                        <span className="bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded">
                          {paymentTypeLabel[op.paymentType] ?? op.paymentType}
                        </span>
                      )}
                      {op.pending && (
                        <span className="bg-orange-50 text-orange-500 px-1.5 py-0.5 rounded">
                          en attente
                        </span>
                      )}
                    </p>
                  </div>
                  <span className={`text-sm font-bold shrink-0 ${op.amount >= 0 ? 'text-ac-green-dark' : 'text-red-600'}`}>
                    {op.amount >= 0 ? '+' : ''}{op.amount.toFixed(2)} €
                  </span>
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>
    </Layout>
  )
}
