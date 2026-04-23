import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { getCatalogs } from '../api/catalogs'
import { Layout } from '../components/Layout'
import { Card } from '../components/ui/Card'

const catalogTypeLabel = ['Commande variable', 'Commande AMAP fixe']

export function CatalogsPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const id = Number(groupId)
  const navigate = useNavigate()

  const { data: catalogs = [], isLoading } = useQuery({
    queryKey: ['catalogs', id],
    queryFn: () => getCatalogs(id),
  })

  return (
    <Layout title="Catalogues">
      {isLoading ? (
        <p className="text-gray-500">Chargement…</p>
      ) : catalogs.length === 0 ? (
        <p className="text-gray-500">Aucun catalogue actif.</p>
      ) : (
        <div className="space-y-3">
          {catalogs.map((c) => (
            <Card key={c.id}>
              <button
                onClick={() => navigate(`/catalogs/${c.id}`)}
                className="w-full text-left px-6 py-4 hover:bg-gray-50 transition-colors rounded-lg"
              >
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <p className="font-medium text-gray-900">{c.name}</p>
                    <p className="text-sm text-gray-500 mt-0.5">
                      {c.vendor?.name && <span>{c.vendor.name} · </span>}
                      {catalogTypeLabel[c.type] ?? 'Catalogue'}
                      {c.feesRate > 0 && <span> · frais {c.feesRate}%</span>}
                    </p>
                  </div>
                  <span className="text-gray-300 text-lg shrink-0">›</span>
                </div>
              </button>
            </Card>
          ))}
        </div>
      )}
    </Layout>
  )
}
