/**
 * Liste des adhérents du groupe.
 *
 * La recherche est en tête parce que c'est ce qu'on vient faire neuf fois sur
 * dix : retrouver quelqu'un. Le solde n'apparaît que s'il n'est pas nul —
 * afficher « 0,00 € » sur soixante lignes ne dit rien à personne.
 */
import { useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { getMembers } from '../../api/members'
import { useAuthStore } from '../../store/auth'

export function AdminMembres() {
  const groupId = useAuthStore((s) => s.currentGroupId)
  const [recherche, setRecherche] = useState('')
  const [page, setPage] = useState(1)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['members', groupId, page, recherche],
    queryFn: () => getMembers(groupId!, page, recherche || undefined),
    enabled: Boolean(groupId),
    // Sans cela, la liste clignote à chaque frappe dans la recherche.
    placeholderData: keepPreviousData,
  })

  if (isLoading && !data) return <Message>Chargement…</Message>
  if (isError) return <Message>La liste des adhérents ne vous est pas accessible.</Message>

  return (
    <div className="flex flex-col">
      <header className="flex items-center justify-between gap-6 bg-surface px-8 py-4">
        <div className="flex flex-col gap-1">
          <h1 className="m-0 font-display text-2xl">Membres</h1>
          <p className="m-0 text-sm text-surface-deep">
            {data?.total ?? 0} adhérents
          </p>
        </div>
        <a
          href="/member/insert"
          className="min-h-[44px] rounded-control bg-action px-5 text-[15px] font-semibold leading-[44px] text-card no-underline"
        >
          Inscrire un adhérent
        </a>
      </header>

      <div className="px-8 pt-5">
        <label className="flex items-center gap-3 rounded-control border-[1.5px] border-line bg-card px-4 py-2.5">
          <svg viewBox="0 0 24 24" className="size-[18px] shrink-0 fill-none stroke-ink-muted" strokeWidth={2} strokeLinecap="round">
            <circle cx="11" cy="11" r="6.5" />
            <path d="M16 16l4 4" />
          </svg>
          <input
            type="search"
            value={recherche}
            placeholder="Chercher un adhérent"
            onChange={(e) => {
              setRecherche(e.target.value)
              setPage(1)
            }}
            className="w-full border-0 bg-transparent text-base text-ink outline-none"
          />
        </label>
      </div>

      <div className="p-8 pt-4">
        <div className="overflow-hidden rounded-card border-[1.5px] border-line bg-card">
          <div className="grid grid-cols-[1.4fr_1.6fr_1fr_120px_90px] gap-3 border-b-[1.5px] border-line bg-tint/60 px-5 py-3 text-xs uppercase tracking-wider text-ink-muted">
            <span>Nom</span>
            <span>Courriel</span>
            <span>Téléphone</span>
            <span className="text-right">Solde</span>
            <span />
          </div>

          {(data?.members ?? []).map((membre) => (
            <div
              key={membre.id}
              className="grid grid-cols-[1.4fr_1.6fr_1fr_120px_90px] items-center gap-3 border-b border-line px-5 py-3 last:border-b-0"
            >
              <span className="flex items-center gap-2 text-base">
                {membre.firstName} {membre.lastName}
                {membre.isManager && (
                  <span className="rounded-[3px] bg-tint px-2 py-0.5 text-xs text-control">responsable</span>
                )}
              </span>
              <span className="truncate text-[15px] text-ink-muted">{membre.email}</span>
              <span className="text-[15px] text-ink-muted">{membre.phone || '—'}</span>
              <span className={`text-right font-display text-base ${membre.balance < 0 ? 'text-action-ink' : ''}`}>
                {membre.balance !== 0 ? euros(membre.balance) : ''}
              </span>
              <Link to={`/refonte/admin/membres/${membre.id}`} className="text-right text-sm text-control no-underline">
                Ouvrir
              </Link>
            </div>
          ))}

          {data?.members.length === 0 && (
            <p className="m-0 px-5 py-6 text-ink-muted">
              {recherche ? `Personne ne correspond à « ${recherche} ».` : 'Aucun adhérent.'}
            </p>
          )}
        </div>

        {data && data.totalPages > 1 && (
          <nav className="mt-4 flex items-center justify-center gap-4">
            <button
              type="button"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
              className="min-h-11 rounded-control border-[1.5px] border-line bg-card px-4 text-[15px] disabled:opacity-40"
            >
              Précédent
            </button>
            <span className="text-[15px] text-ink-muted">
              Page {data.page} sur {data.totalPages}
            </span>
            <button
              type="button"
              disabled={page >= data.totalPages}
              onClick={() => setPage((p) => p + 1)}
              className="min-h-11 rounded-control border-[1.5px] border-line bg-card px-4 text-[15px] disabled:opacity-40"
            >
              Suivant
            </button>
          </nav>
        )}
      </div>
    </div>
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center px-8 text-center text-ink-muted">{children}</div>
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}
