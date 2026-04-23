import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getMe, updateMe } from '../api/users'
import { Layout } from '../components/Layout'
import { Card, CardHeader } from '../components/ui/Card'
import { Button } from '../components/ui/Button'

export function ProfilePage() {
  const qc = useQueryClient()

  const { data: user } = useQuery({ queryKey: ['me'], queryFn: getMe })

  const [form, setForm] = useState({
    firstName: '', lastName: '', phone: '', address1: '', zipCode: '', city: '',
  })
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (user) setForm({
      firstName: user.firstname ?? '',
      lastName: user.lastname ?? '',
      phone: (user as any).phone ?? '',
      address1: (user as any).address1 ?? '',
      zipCode: (user as any).zipCode ?? '',
      city: (user as any).city ?? '',
    })
  }, [user])

  const mutation = useMutation({
    mutationFn: () => updateMe({
      firstName: form.firstName,
      lastName: form.lastName,
      phone: form.phone || undefined,
      address1: form.address1 || undefined,
      zipCode: form.zipCode || undefined,
      city: form.city || undefined,
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['me'] })
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
      <div className="max-w-lg space-y-6">
        <Card>
          <CardHeader title="Informations personnelles" />
          <div className="px-6 py-4 space-y-4">
            <div className="grid grid-cols-2 gap-4">
              {field('Prénom', 'firstName')}
              {field('Nom', 'lastName')}
            </div>
            {field('Téléphone', 'phone', 'tel')}
            {field('Adresse', 'address1')}
            <div className="grid grid-cols-2 gap-4">
              {field('Code postal', 'zipCode')}
              {field('Ville', 'city')}
            </div>
          </div>
          <div className="px-6 py-4 border-t border-gray-100 flex items-center justify-between">
            {saved && <span className="text-sm text-ac-green-dark">Modifications enregistrées ✓</span>}
            {mutation.isError && <span className="text-sm text-red-600">Erreur lors de la sauvegarde</span>}
            {!saved && !mutation.isError && <span />}
            <Button onClick={() => mutation.mutate()} loading={mutation.isPending}>
              Enregistrer
            </Button>
          </div>
        </Card>

        <Card>
          <div className="px-6 py-4">
            <p className="text-sm text-gray-500">Email</p>
            <p className="font-medium text-gray-900 mt-0.5">{user?.email}</p>
          </div>
        </Card>
      </div>
    </Layout>
  )
}
