import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useSearchParams } from 'react-router-dom'
import { getOrders, saveOrders } from '../api/orders'
import { getCatalog } from '../api/catalogs'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'
import { Button } from '../components/ui/Button'

export function OrdersPage() {
  const { groupId } = useParams<{ groupId: string }>()
  const [searchParams] = useSearchParams()
  const gid = Number(groupId)
  const distributionId = Number(searchParams.get('distributionId'))
  const catalogId = Number(searchParams.get('catalogId')) || undefined
  const qc = useQueryClient()

  const [quantities, setQuantities] = useState<Record<number, number>>({})
  const [saved, setSaved] = useState(false)

  const { data: catalog } = useQuery({
    queryKey: ['catalog', catalogId],
    queryFn: () => getCatalog(catalogId!),
    enabled: !!catalogId,
  })

  useQuery({
    queryKey: ['orders', distributionId, catalogId],
    queryFn: () => getOrders(distributionId, catalogId),
    enabled: !!distributionId,
    select: (orders) => {
      const init: Record<number, number> = {}
      orders.forEach((o) => { init[o.productId] = o.quantity })
      setQuantities(init)
      return orders
    },
  })

  const mutation = useMutation({
    mutationFn: () =>
      saveOrders({
        distributionId,
        catalogId,
        orders: Object.entries(quantities)
          .map(([pid, qty]) => ({ productId: Number(pid), quantity: qty }))
          .filter((o) => o.quantity > 0),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['orders', distributionId, catalogId] })
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    },
  })

  const total = catalog?.products.reduce((sum, p) => {
    const qty = quantities[p.id] ?? 0
    const sub = qty * p.price
    return sum + sub + sub * ((catalog.feesRate ?? 0) / 100)
  }, 0) ?? 0

  const setQty = (productId: number, value: number) => {
    setSaved(false)
    setQuantities((prev) => ({ ...prev, [productId]: Math.max(0, value) }))
  }

  if (!distributionId) {
    return (
      <Layout title="Commandes">
        <p className="text-gray-500">Aucune distribution sélectionnée.</p>
      </Layout>
    )
  }

  return (
    <Layout title={catalog?.name ?? 'Commandes'} backTo={`/groups/${gid}/distributions`} backLabel="Distributions">
      {!catalog ? (
        <p className="text-gray-500">Chargement…</p>
      ) : (
        <div className="space-y-4">
          <Card>
            <CardHeader
              title={catalog.name}
              subtitle={catalog.feesRate ? `Frais de service : ${catalog.feesRate}%` : undefined}
            />
            <div className="divide-y divide-gray-100">
              {catalog.products.map((p) => {
                const qty = quantities[p.id] ?? 0
                const sub = qty * p.price
                return (
                  <div key={p.id} className="px-6 py-4 flex items-center justify-between gap-4">
                    <div className="flex-1 min-w-0">
                      <p className="font-medium text-gray-900">{p.name}</p>
                      <p className="text-sm text-gray-500">
                        {p.price.toFixed(2)} € / {p.unitLabel ?? 'unité'}
                        {p.stock !== undefined && p.stock !== null && (
                          <span className="ml-2 text-gray-400">— stock : {p.stock}</span>
                        )}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <button
                        onClick={() => setQty(p.id, qty - 1)}
                        className="w-8 h-8 rounded-full border border-gray-300 text-gray-600
                          hover:bg-gray-100 flex items-center justify-center text-lg"
                      >−</button>
                      <span className="w-6 text-center font-medium">{qty}</span>
                      <button
                        onClick={() => setQty(p.id, qty + 1)}
                        className="w-8 h-8 rounded-full border border-gray-300 text-gray-600
                          hover:bg-gray-100 flex items-center justify-center text-lg"
                      >+</button>
                      <span className={`w-16 text-right text-sm font-semibold ${sub > 0 ? 'text-ac-green-dark' : 'text-gray-300'}`}>
                        {sub > 0 ? `${sub.toFixed(2)} €` : '—'}
                      </span>
                    </div>
                  </div>
                )
              })}
            </div>
          </Card>

          {/* Total */}
          <div className="bg-white rounded-lg border border-gray-200 px-6 py-4 flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-500">Total</p>
              <p className="text-2xl font-bold text-gray-900">{total.toFixed(2)} €</p>
            </div>
            <div className="flex items-center gap-3">
              {saved && <span className="text-sm text-ac-green-dark">Enregistré ✓</span>}
              {mutation.isError && <span className="text-sm text-red-600">Erreur</span>}
              <Button onClick={() => mutation.mutate()} loading={mutation.isPending} disabled={total === 0}>
                Valider la commande
              </Button>
            </div>
          </div>
        </div>
      )}
    </Layout>
  )
}
