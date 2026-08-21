/**
 * Le compte de l'adhérent : ses coordonnées, son groupe, et les accès qui en
 * découlent. Rien d'autre — ce qui touche à l'administration du groupe se
 * trouve ailleurs, même pour un responsable.
 */
import { useQuery } from '@tanstack/react-query'
import { fetchHome } from '../../api/home'
import { useAuthStore } from '../../store/auth'

export function CompteRefonte() {
  const utilisateur = useAuthStore((s) => s.user)
  const deconnexion = useAuthStore((s) => s.logout)
  const { data } = useQuery({ queryKey: ['home', 0], queryFn: () => fetchHome(0) })

  const initiales = [utilisateur?.firstname, utilisateur?.lastname]
    .filter(Boolean)
    .map((mot) => mot!.charAt(0).toUpperCase())
    .join('')

  return (
    <div className="flex flex-col text-ink">
      <header className="flex items-center gap-4 bg-surface px-5 py-6">
        <span className="flex size-[62px] items-center justify-center rounded-full border-[1.5px] border-surface-deep bg-card font-display text-2xl">
          {initiales || '—'}
        </span>
        <span className="flex flex-col gap-0.5">
          <span className="font-display text-2xl">
            {utilisateur?.firstname} {utilisateur?.lastname}
          </span>
          {data?.groupName && <span className="text-sm text-control">{data.groupName}</span>}
        </span>
      </header>

      <section className="flex flex-col gap-4 px-5 py-5">
        <Bloc titre="Mes coordonnées">
          <Champ libelle="Courriel" valeur={utilisateur?.email ?? '—'} />
        </Bloc>

        <Bloc titre="Mon groupe">
          <Lien libelle="Calendrier des permanences" href="/distribution/volunteersCalendar" />
          <Lien libelle="Écrire au responsable" href="/messages" />
          <Lien libelle="Documents du groupe" href="/amap" />
        </Bloc>

        <button
          type="button"
          onClick={deconnexion}
          className="self-center bg-transparent p-0 text-[15px] text-ink-muted underline"
        >
          Se déconnecter
        </button>
      </section>
    </div>
  )
}

function Bloc({ titre, children }: { titre: string; children: React.ReactNode }) {
  return (
    <section className="overflow-hidden rounded-card border-[1.5px] border-line bg-card">
      <h2 className="m-0 border-b border-line px-4 py-3.5 font-display text-[17px] italic">{titre}</h2>
      {children}
    </section>
  )
}

function Champ({ libelle, valeur }: { libelle: string; valeur: string }) {
  return (
    <div className="flex flex-col gap-0.5 border-b border-line px-4 py-3 last:border-b-0">
      <span className="text-xs text-ink-muted">{libelle}</span>
      <span className="text-[15px]">{valeur}</span>
    </div>
  )
}

/* Ces pages vivent encore côté Go : un lien classique, et non une route du
 * routeur, sinon la SPA tenterait de les résoudre et rendrait une page vide. */
function Lien({ libelle, href }: { libelle: string; href: string }) {
  return (
    <a href={href} className="flex items-center justify-between border-b border-line px-4 py-4 text-base text-ink no-underline last:border-b-0">
      {libelle}
      <svg viewBox="0 0 24 24" className="size-[17px] fill-none stroke-ink-faint" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
        <path d="M9 6l6 6-6 6" />
      </svg>
    </a>
  )
}
