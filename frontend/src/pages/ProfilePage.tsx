import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { fetchAccount, updateAccount } from '../api/account'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'
import { Button } from '../components/ui/Button'

export function ProfilePage() {
  const qc = useQueryClient()
  const { data: account } = useQuery({ queryKey: ['account'], queryFn: fetchAccount })
  const user = account?.user

  const [form, setForm] = useState({
    firstName: '', lastName: '', phone: '', address1: '', zipCode: '', city: '',
  })
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (user) setForm({
      firstName: user.firstName ?? '',
      lastName: user.lastName ?? '',
      phone: user.phone ?? '',
      address1: user.address1 ?? '',
      zipCode: user.zipCode ?? '',
      city: user.city ?? '',
    })
  }, [user])

  const mutation = useMutation({
    mutationFn: () => updateAccount({
      firstName: form.firstName,
      lastName: form.lastName,
      phone: form.phone || undefined,
      address1: form.address1 || undefined,
      zipCode: form.zipCode || undefined,
      city: form.city || undefined,
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['account'] })
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    },
  })

  const field = (label: string, key: keyof typeof form, type = 'text') => (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">{label}</label>
      <input
        type={type}
        value={form[key]}
        onChange={(e) => { setSaved(false); setForm((f) => ({ ...f, [key]: e.target.value })) }}
        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm
          focus:outline-none focus:ring-2 focus:ring-ac-green focus:border-transparent"
      />
    </div>
  )

  return (
    <Layout title="Mon profil">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Colonne principale : commandes + souscriptions */}
        <div className="md:col-span-2 space-y-6">
          <Card>
            <CardHeader title="Dernières commandes" subtitle="30 derniers jours" />
            {!account ? (
              <p className="px-6 py-4 text-sm text-gray-500">Chargement…</p>
            ) : account.recentOrders.length === 0 ? (
              <p className="px-6 py-4 text-sm text-gray-500">Aucune commande récente.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-gray-400 border-b border-gray-200">
                      <th className="text-left font-normal py-2 px-6">Produit</th>
                      <th className="text-left font-normal py-2">Qté</th>
                      <th className="text-right font-normal py-2 px-6">Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {account.recentOrders.map((o, i) => (
                      <tr key={i} className="border-b border-gray-100">
                        <td className="py-2 px-6 text-ac-green-dark">{o.productName}</td>
                        <td className="py-2">{o.smartQty}</td>
                        <td className="py-2 px-6 text-right">{o.total.toFixed(2)} €</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>

          {account && account.subscriptions.length > 0 && (
            <Card>
              <CardHeader title="Contrats AMAP" />
              <div className="px-6 py-4 space-y-2">
                {account.subscriptions.map((s, i) => (
                  <p key={i} className="text-sm text-gray-700">
                    <b>{s.catalogName}</b> — du {s.startDate}
                    {s.endDate && <> au {s.endDate}</>}
                  </p>
                ))}
              </div>
            </Card>
          )}
        </div>

        {/* Sidebar : compte + édition + alertes */}
        <aside className="space-y-4">
          {account?.membershipRenewalPeriod && (
            <Card className="bg-red-50 border-red-200">
              <div className="px-5 py-4 text-sm text-red-700">
                <i className="icon-alert" aria-hidden="true" />{' '}
                Cotisation à renouveler pour la période{' '}
                <b>{account.membershipRenewalPeriod}</b>
              </div>
            </Card>
          )}

          <Card>
            <CardHeader title="Compte" />
            <div className="px-5 py-4 text-sm space-y-1">
              {user && (
                <>
                  <p>
                    <i className="icon-user text-gray-400" aria-hidden="true" />{' '}
                    <b>{user.firstName} {user.lastName}</b>
                  </p>
                  <p>
                    <i className="icon-mail text-gray-400" aria-hidden="true" />{' '}
                    <a href={`mailto:${user.email}`} className="text-ac-green-dark hover:underline">
                      {user.email}
                    </a>
                  </p>
                  {user.phone && (
                    <p>
                      <i className="icon-phone text-gray-400" aria-hidden="true" />{' '}
                      {user.phone}
                    </p>
                  )}
                </>
              )}
            </div>
          </Card>

          <Card>
            <CardHeader title="Modifier mes informations" />
            <div className="px-5 py-4 space-y-3">
              <div className="grid grid-cols-2 gap-3">
                {field('Prénom', 'firstName')}
                {field('Nom', 'lastName')}
              </div>
              {field('Téléphone', 'phone', 'tel')}
              {field('Adresse', 'address1')}
              <div className="grid grid-cols-2 gap-3">
                {field('Code postal', 'zipCode')}
                {field('Ville', 'city')}
              </div>
            </div>
            <div className="px-5 py-3 border-t border-gray-100 flex items-center justify-between">
              {saved && <span className="text-sm text-ac-green-dark">Enregistré ✓</span>}
              {mutation.isError && <span className="text-sm text-red-600">Erreur</span>}
              {!saved && !mutation.isError && <span />}
              <Button onClick={() => mutation.mutate()} loading={mutation.isPending}>
                Enregistrer
              </Button>
            </div>
          </Card>
        </aside>
      </div>
    </Layout>
  )
}
