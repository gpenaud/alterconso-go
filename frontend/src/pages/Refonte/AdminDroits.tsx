/**
 * Qui peut quoi dans le groupe.
 *
 * L'écran reflète le modèle en vigueur : un responsable de groupe qui a tout,
 * quatre délégations qui n'ouvrent qu'un écran chacune, et un responsable
 * technique qui tient son rôle de la configuration — non modifiable ici, d'où
 * le cadenas.
 *
 * Le panneau de droite explique chaque droit en une phrase. Sans lui, « gestion
 * des paramètres » ne dit pas ce qu'on confie réellement.
 */
import { useQuery } from '@tanstack/react-query'
import { fetchRightHolders } from '../../api/adminRights'

const explications: Array<{ titre: string; texte: string }> = [
  {
    titre: 'Responsable de groupe',
    texte:
      "Tout, y compris confier les droits. Un seul par groupe ; c'est le responsable technique qui le désigne.",
  },
  {
    titre: 'Gestion des distributions',
    texte: 'Le calendrier et les producteurs qui y participent.',
  },
  {
    titre: 'Gestion des paramètres',
    texte: 'Identité du groupe, adhésions, monnaie, documents.',
  },
  { titre: 'Gestion des membres', texte: "Inscriptions, coordonnées, liste d'attente." },
  { titre: 'Gestion des catalogues', texte: 'Produits et prix, catalogue par catalogue.' },
  { titre: 'Messages', texte: 'Écrire à tout le groupe depuis la messagerie.' },
]

export function AdminDroits() {
  const { data, isLoading, isError } = useQuery({ queryKey: ['admin-rights'], queryFn: fetchRightHolders })

  if (isLoading) return <Message>Chargement…</Message>
  if (isError) return <Message>Seuls les responsables peuvent consulter les droits.</Message>

  return (
    <div className="flex flex-col">
      <header className="flex items-center justify-between gap-6 bg-surface px-8 py-4">
        <h1 className="m-0 font-display text-2xl">Droits d'administration</h1>
        <a
          href="/amapadmin/rights/add"
          className="min-h-[44px] rounded-control bg-action px-5 text-[15px] font-semibold leading-[44px] text-card no-underline"
        >
          Confier un droit
        </a>
      </header>

      <div className="flex items-start gap-6 p-8">
        <div className="grow overflow-hidden rounded-card border-[1.5px] border-line bg-card">
          {(data ?? []).map((porteur) => (
            <div
              key={porteur.userId}
              className="flex items-center gap-4 border-b border-line px-5 py-4 last:border-b-0"
            >
              <span className="flex w-[220px] shrink-0 flex-col gap-0.5">
                <span className="text-base">{porteur.name}</span>
                {porteur.role && (
                  <span className={`text-xs font-semibold ${porteur.editable ? 'text-control' : 'text-action-ink'}`}>
                    {porteur.role}
                  </span>
                )}
              </span>

              <span className="grow text-[15px] text-ink-muted">
                {porteur.role && !porteur.editable
                  ? 'Tous les droits, sur tous les groupes — défini dans la configuration.'
                  : porteur.role
                    ? 'Toutes les délégations lui sont acquises.'
                    : porteur.delegations.join(' · ')}
              </span>

              {porteur.editable ? (
                <a href={`/amapadmin/rights/edit/${porteur.userId}`} className="shrink-0 text-sm text-control no-underline">
                  Modifier
                </a>
              ) : (
                <svg viewBox="0 0 24 24" className="size-[17px] shrink-0 fill-none stroke-ink-faint" strokeWidth={1.8} strokeLinecap="round">
                  <rect x="5" y="11" width="14" height="9" rx="1.6" />
                  <path d="M8 11V8a4 4 0 0 1 8 0v3" />
                </svg>
              )}
            </div>
          ))}

          {data?.length === 0 && (
            <p className="m-0 px-5 py-6 text-ink-muted">Personne ne détient de droit dans ce groupe.</p>
          )}
        </div>

        <aside className="w-[360px] shrink-0 rounded-card border-[1.5px] border-line bg-card p-5">
          <h2 className="m-0 mb-3.5 font-display text-lg italic">Qui peut quoi</h2>
          <dl className="m-0 flex flex-col gap-3 text-sm leading-relaxed">
            {explications.map((droit, i) => (
              <div key={droit.titre} className={i === 1 ? 'border-t border-line pt-3' : ''}>
                <dt className="font-semibold">{droit.titre}</dt>
                <dd className="m-0 text-ink-muted">{droit.texte}</dd>
              </div>
            ))}
          </dl>
        </aside>
      </div>
    </div>
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center px-8 text-center text-ink-muted">{children}</div>
}
