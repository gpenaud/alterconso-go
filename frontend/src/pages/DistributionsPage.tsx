import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { getDistributions } from '../api/distributions'
import { Layout } from '../components/Layout'
import { Card } from '../components/ui/Card'

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('fr-FR', {
    weekday: 'long', day: 'numeric', month: 'long',
  })
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
}

export function DistributionsPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const id = Number(groupId)
  const navigate = useNavigate()

  const { data: distribs = [], isLoading } = useQuery({
    queryKey: ['distributions', id],
    queryFn: () => getDistributions(id),
  })

  return (
    <Layout title="Distributions">
      {isLoading ? (
        <p className="text-gray-500">Chargement…</p>
      ) : distribs.length === 0 ? (
        <p className="text-gray-500">Aucune distribution à venir.</p>
      ) : (
        <div className="space-y-4">
          {distribs.map((md) => {
            const canOrder = md.orderEndDate
              ? new Date() < new Date(md.orderEndDate)
              : true

            return (
              <Card key={md.id}>
                <div className="px-6 py-4">
                  <div className="flex items-start justify-between gap-4 flex-wrap">
                    <div>
                      <p className="font-semibold text-gray-900 capitalize">
                        {formatDate(md.distribStartDate)}
                      </p>
                      <p className="text-sm text-gray-500 mt-0.5">
                        {formatTime(md.distribStartDate)} – {formatTime(md.distribEndDate)}
                        {' · '}{md.place.name}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      {md.validated && (
                        <span className="text-xs bg-gray-100 text-gray-500 px-2 py-0.5 rounded-full">
                          Validée
                        </span>
                      )}
                      {canOrder && !md.validated && (
                        <span className="text-xs bg-green-50 text-ac-green-dark px-2 py-0.5 rounded-full">
                          Commandes ouvertes
                        </span>
                      )}
                    </div>
                  </div>

                  {md.distributions.length > 0 && (
                    <div className="mt-3 flex flex-wrap gap-2">
                      {md.distributions.map((d) => (
                        <button
                          key={d.id}
                          onClick={() =>
                            navigate(`/groups/${id}/orders?distributionId=${md.id}&catalogId=${d.catalogId}`)
                          }
                          className="text-xs bg-white border border-gray-200 hover:border-ac-green
                            text-gray-700 px-3 py-1 rounded-full transition-colors"
                        >
                          {d.catalog.name}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </Card>
            )
          })}
        </div>
      )}
    </Layout>
  )
}
