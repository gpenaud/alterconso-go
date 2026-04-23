import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { getGroups } from '../api/groups'
import { useAuthStore } from '../store/auth'
import { Card } from '../components/ui/Card'

export function GroupsPage() {
  const navigate = useNavigate()
  const { user, setGroup, logout } = useAuthStore()

  const { data: groups = [], isLoading } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
  })

  const handleSelect = (groupId: number) => {
    setGroup(groupId)
    navigate(`/groups/${groupId}`)
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between">
        <h1 className="text-xl font-bold text-ac-green-dark">Alterconso</h1>
        <div className="flex items-center gap-4">
          <span className="text-sm text-gray-600">{user?.firstname} {user?.lastname}</span>
          <button onClick={logout} className="text-sm text-gray-500 hover:text-gray-700">
            Déconnexion
          </button>
        </div>
      </header>

      <main className="max-w-2xl mx-auto px-4 py-10">
        <h2 className="text-2xl font-semibold text-gray-900 mb-6">Mes groupes</h2>

        {isLoading ? (
          <p className="text-gray-500">Chargement…</p>
        ) : groups.length === 0 ? (
          <p className="text-gray-500">Vous n'êtes membre d'aucun groupe.</p>
        ) : (
          <div className="space-y-3">
            {groups.map((g) => (
              <Card key={g.id}>
                <button
                  onClick={() => handleSelect(g.id)}
                  className="w-full text-left px-6 py-4 hover:bg-gray-50 transition-colors rounded-lg"
                >
                  <div className="font-medium text-gray-900">{g.name}</div>
                  {g.txtIntro && (
                    <div className="text-sm text-gray-500 mt-1 line-clamp-2">{g.txtIntro}</div>
                  )}
                </button>
              </Card>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
