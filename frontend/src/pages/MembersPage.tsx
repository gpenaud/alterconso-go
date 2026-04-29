import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { getMembers } from '../api/members'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'

export function MembersPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const id = Number(groupId)
  const [page, setPage] = useState(1)

  const { data, isLoading } = useQuery({
    queryKey: ['members', id, page],
    queryFn: () => getMembers(id, page),
    placeholderData: (prev) => prev,
  })

  const members = data?.members ?? []
  const totalPages = data?.totalPages ?? 1

  return (
    <Layout title="Membres">
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <div className="md:col-span-3">
          <Card>
            <CardHeader
              title="Membres du groupe"
              subtitle={data ? `${data.total} membre(s)` : undefined}
            />
            {isLoading ? (
              <p className="px-6 py-4 text-sm text-gray-500">Chargement…</p>
            ) : members.length === 0 ? (
              <p className="px-6 py-4 text-sm text-gray-500">Aucun membre.</p>
            ) : (
              <div className="divide-y divide-gray-100">
                {members.map((m) => {
                  const balanceColor = m.balance >= 0 ? 'text-ac-green-dark' : 'text-red-600'
                  return (
                    <div
                      key={m.id}
                      className="px-6 py-3 flex items-center justify-between gap-4"
                    >
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-gray-900">
                          {m.lastName.toUpperCase()} {m.firstName}
                        </p>
                        <p className="text-xs text-gray-400 mt-0.5 truncate">
                          {m.address || m.email}
                        </p>
                      </div>
                      <div className="flex items-center gap-3 shrink-0">
                        {m.isManager && (
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

            {totalPages > 1 && (
              <div className="flex items-center justify-center gap-2 px-6 py-4 border-t border-gray-100">
                <button
                  type="button"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page <= 1}
                  className="px-3 py-1.5 rounded border border-gray-300 text-gray-600 disabled:opacity-40 hover:bg-gray-50"
                  aria-label="Page précédente"
                >
                  <i className="icon-chevron-left" aria-hidden="true" />
                </button>
                <span className="px-3 py-1.5 text-sm text-gray-600 select-none">
                  Page {page} / {totalPages}
                </span>
                <button
                  type="button"
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page >= totalPages}
                  className="px-3 py-1.5 rounded border border-gray-300 text-gray-600 disabled:opacity-40 hover:bg-gray-50"
                  aria-label="Page suivante"
                >
                  <i className="icon-chevron-right" aria-hidden="true" />
                </button>
              </div>
            )}
          </Card>
        </div>

        {/* Sidebar : listes annexes */}
        <aside className="space-y-4">
          <Card>
            <CardHeader title="Listes" />
            <ul className="px-4 py-3 space-y-2 text-sm">
              <li className="text-gray-700">
                Membres du groupe ({data?.total ?? '…'})
              </li>
              <li>
                <a
                  href="/member/waiting"
                  className="text-ac-green-dark hover:underline"
                >
                  Liste d'attente ({data?.waitingListCount ?? '…'})
                </a>
              </li>
            </ul>
          </Card>
        </aside>
      </div>
    </Layout>
  )
}
