import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { getMembers } from '../api/members'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'

export function MembersPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const id = Number(groupId)

  const { data: members = [], isLoading } = useQuery({
    queryKey: ['members', id],
    queryFn: () => getMembers(id),
  })

  return (
    <Layout title="Membres">
      <Card>
        <CardHeader title="Membres du groupe" subtitle={`${members.length} membre(s)`} />
        {isLoading ? (
          <p className="px-6 py-4 text-sm text-gray-500">Chargement…</p>
        ) : members.length === 0 ? (
          <p className="px-6 py-4 text-sm text-gray-500">Aucun membre.</p>
        ) : (
          <div className="divide-y divide-gray-100">
            {members.map((m) => {
              const balanceColor = m.balance >= 0 ? 'text-ac-green-dark' : 'text-red-600'
              return (
                <div key={m.id} className="px-6 py-3 flex items-center justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900">
                      {m.lastName.toUpperCase()} {m.firstName}
                    </p>
                    <p className="text-xs text-gray-400 mt-0.5 truncate">{m.email}</p>
                  </div>
                  <div className="flex items-center gap-3 shrink-0">
                    {m.rights?.includes('hasRight') && (
                      <span className="text-xs bg-blue-50 text-blue-600 px-2 py-0.5 rounded-full">
                        Admin
                      </span>
                    )}
                    <span className={`text-sm font-semibold ${balanceColor}`}>
                      {m.balance >= 0 ? '+' : ''}{m.balance.toFixed(2)} €
                    </span>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </Card>
    </Layout>
  )
}
