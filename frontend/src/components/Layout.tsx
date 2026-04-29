import { Link, useParams } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useAuthStore } from '../store/auth'
import { useDocumentTitle } from '../utils/useDocumentTitle'

interface Props {
  children: ReactNode
  title: string
  backTo?: string
  backLabel?: string
}

const navItems = [
  { label: 'Accueil', path: '' },
  { label: 'Distributions', path: '/distributions' },
  { label: 'Finances', path: '/finances' },
  { label: 'Membres', path: '/members' },
  { label: 'Catalogues', path: '/catalogs' },
  { label: 'Admin', path: '/admin' },
]

export function Layout({ children, title, backTo, backLabel }: Props) {
  const { groupId } = useParams<{ groupId: string }>()
  const { user } = useAuthStore()
  useDocumentTitle(title)

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Top bar */}
      <header>
        <div className="max-w-5xl mx-auto px-4 h-14 flex items-center justify-between">
          <Link to={backTo ?? '/groups'} className="flex items-center gap-1.5 text-gray-500 hover:text-gray-700 text-sm font-medium">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="15 18 9 12 15 6" />
            </svg>
            {backLabel ?? 'Accueil'}
          </Link>

          <a href="#" className="flex items-center gap-1.5 text-gray-500 hover:text-gray-700 text-sm font-medium">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <circle cx="12" cy="12" r="10" />
              <text x="12" y="16.5" textAnchor="middle" fill="white" fontSize="13" fontWeight="bold" fontFamily="sans-serif">i</text>
            </svg>
            Aide
          </a>

          <Link to="/profile" className="flex items-center gap-1.5 text-gray-500 hover:text-gray-700 text-sm font-medium">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <circle cx="12" cy="8" r="4" />
              <path d="M4 20c0-4 3.6-7 8-7s8 3 8 7" />
            </svg>
            {user?.lastname?.toUpperCase()} {user?.firstname}
          </Link>
        </div>

        {/* Navigation secondaire (si on est dans un groupe) */}
        {groupId && (
          <nav className="max-w-5xl mx-auto px-4 flex gap-1 overflow-x-auto">
            {navItems.map((item) => {
              const to = `/groups/${groupId}${item.path}`
              const active = window.location.pathname === to
              return (
                <Link
                  key={item.path}
                  to={to}
                  className={`px-3 py-2 text-sm font-medium border-b-2 whitespace-nowrap transition-colors
                    ${active
                      ? 'border-ac-green text-ac-green-dark'
                      : 'border-transparent text-gray-500 hover:text-gray-700'
                    }`}
                >
                  {item.label}
                </Link>
              )
            })}
          </nav>
        )}
      </header>

      <main className="max-w-5xl mx-auto px-4 py-6">
        {children}
      </main>
    </div>
  )
}
