import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { getGroupFinances, validateDistribution } from '../api/admin'
import { getDistributions } from '../api/distributions'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'
import { Button } from '../components/ui/Button'

export function AdminPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const id = Number(groupId)
  const qc = useQueryClient()

  const { data: finances, isError: financeError } = useQuery({
    queryKey: ['finances', id],
    queryFn: () => getGroupFinances(id),
  })

  const { data: distribs = [] } = useQuery({
    queryKey: ['distributions', id],
    queryFn: () => getDistributions(id),
  })

  const validateMutation = useMutation({
    mutationFn: (distribId: number) => validateDistribution(distribId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['distributions', id] }),
  })

  if (financeError) {
    return (
      <Layout title="Administration">
        <p className="text-red-600 text-sm">Accès refusé — réservé aux administrateurs du groupe.</p>
      </Layout>
    )
  }

  const toValidate = distribs.filter((d) => !d.validated && new Date(d.distribEndDate) < new Date())

  return (
    <Layout title="Administration">
      <div className="space-y-6">

        {/* Résumé financier */}
        {finances && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            {[
              { label: 'Total dettes', value: finances.totalDebt, color: 'text-red-600' },
              { label: 'Total crédits', value: finances.totalCredit, color: 'text-ac-green-dark' },
              { label: 'Paiements en attente', value: finances.pendingSum, color: 'text-orange-500' },
              { label: 'Nb. en attente', value: finances.pendingCount, color: 'text-gray-700', unit: '' },
            ].map((s) => (
              <Card key={s.label}>
                <div className="px-4 py-3">
                  <p className="text-xs text-gray-500">{s.label}</p>
                  <p className={`text-xl font-bold mt-1 ${s.color}`}>
                    {s.unit !== '' ? `${(s.value as number).toFixed(2)} €` : s.value}
                  </p>
                </div>
              </Card>
            ))}
          </div>
        )}

        {/* Distributions à valider */}
        {toValidate.length > 0 && (
          <Card>
            <CardHeader
              title="Distributions à valider"
              subtitle={`${toValidate.length} distribution(s) passée(s) non validée(s)`}
            />
            <div className="divide-y divide-gray-100">
              {toValidate.map((d) => (
                <div key={d.id} className="px-6 py-3 flex items-center justify-between gap-4">
                  <div>
                    <p className="text-sm font-medium text-gray-900">
                      {new Date(d.distribStartDate).toLocaleDateString('fr-FR', {
                        weekday: 'long', day: 'numeric', month: 'long',
                      })}
                    </p>
                    <p className="text-xs text-gray-400">{d.place.name}</p>
                  </div>
                  <Button
                    variant="secondary"
                    loading={validateMutation.isPending}
                    onClick={() => validateMutation.mutate(d.id)}
                  >
                    Valider
                  </Button>
                </div>
              ))}
            </div>
          </Card>
        )}

        {/* Balances des membres */}
        {finances && (
          <Card>
            <CardHeader
              title="Balances des membres"
              subtitle={`${finances.members.length} membre(s)`}
            />
            <div className="divide-y divide-gray-100">
              {finances.members.map((m) => (
                <div key={m.userId} className="px-6 py-3 flex items-center justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900">{m.name}</p>
                    <p className="text-xs text-gray-400 truncate">{m.email}</p>
                  </div>
                  <span className={`text-sm font-bold shrink-0 ${m.balance >= 0 ? 'text-ac-green-dark' : 'text-red-600'}`}>
                    {m.balance >= 0 ? '+' : ''}{m.balance.toFixed(2)} €
                  </span>
                </div>
              ))}
            </div>
          </Card>
        )}
      </div>
    </Layout>
  )
}
