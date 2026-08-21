/**
 * Ossature des écrans adhérents : le contenu, puis la barre de navigation.
 *
 * Trois entrées, pas plus. Tout ce qui relève de l'administration vit
 * ailleurs : un adhérent n'a pas à croiser des écrans qu'il ne peut pas
 * ouvrir, et un responsable sait où les trouver.
 */
import { NavLink } from 'react-router-dom'

export function LayoutRefonte({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col bg-canvas">
      <div className="grow">{children}</div>

      <nav className="flex justify-around border-t-[1.5px] border-line bg-card pb-5 pt-3">
        <Onglet to="/refonte" libelle="Accueil" icone={<IconeMaison />} />
        <Onglet to="/refonte/commandes" libelle="Commandes" icone={<IconeTicket />} />
        <Onglet to="/refonte/compte" libelle="Compte" icone={<IconePersonne />} />
      </nav>
    </div>
  )
}

function Onglet({ to, libelle, icone }: { to: string; libelle: string; icone: React.ReactNode }) {
  return (
    <NavLink
      to={to}
      end
      // `end` sur toutes : sans lui, « Accueil » resterait actif sur chaque
      // sous-route, et deux onglets s allumeraient en même temps.
      className={({ isActive }) =>
        `flex min-h-11 flex-col items-center justify-center gap-1 no-underline ${
          isActive ? 'text-control' : 'text-ink-faint'
        }`
      }
    >
      {icone}
      <span className="text-[11px] uppercase tracking-wide">{libelle}</span>
    </NavLink>
  )
}

const traits = 'size-[21px] fill-none stroke-current'

function IconeMaison() {
  return (
    <svg viewBox="0 0 24 24" className={traits} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 11l8-6 8 6" />
      <path d="M6 10v9h12v-9" />
    </svg>
  )
}

function IconeTicket() {
  return (
    <svg viewBox="0 0 24 24" className={traits} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 4h12v16l-6-3-6 3z" />
    </svg>
  )
}

function IconePersonne() {
  return (
    <svg viewBox="0 0 24 24" className={traits} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="8" r="3.5" />
      <path d="M5 20c0-3.5 3.1-5.5 7-5.5s7 2 7 5.5" />
    </svg>
  )
}
