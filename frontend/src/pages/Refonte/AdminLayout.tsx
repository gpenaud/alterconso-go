/**
 * Ossature de l'administration.
 *
 * Le vert profond en colonne, là où l'espace adhérent le garde pour de menus
 * détails : on sait au premier coup d'œil de quel côté on se trouve, sans
 * changer de vocabulaire graphique.
 *
 * Les entrées ne sont pas filtrées par les droits ici — le serveur refuse ce
 * qu'il faut refuser. Elles le seront quand chaque écran existera, pour ne pas
 * proposer des portes fermées.
 */
import { NavLink } from 'react-router-dom'

const entrees = [
  { to: '/refonte/admin', libelle: 'Tableau de bord' },
  { to: '/refonte/admin/distributions', libelle: 'Distributions' },
  { to: '/refonte/admin/commandes', libelle: 'Commandes' },
  { to: '/refonte/admin/catalogues', libelle: 'Catalogues' },
  { to: '/refonte/admin/membres', libelle: 'Membres' },
  { to: '/refonte/admin/droits', libelle: 'Droits' },
]

export function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen bg-canvas text-ink">
      <nav className="flex w-[240px] shrink-0 flex-col gap-0.5 bg-surface-deep py-6 text-[#e8f0d8]">
        <span className="px-[22px] pb-6 font-display text-xl italic text-card">Administration</span>
        {entrees.map((entree) => (
          <NavLink
            key={entree.to}
            to={entree.to}
            end
            className={({ isActive }) =>
              `px-[22px] py-3 text-[15px] no-underline ${
                isActive
                  ? 'border-l-[3px] border-surface bg-white/10 text-card'
                  : 'text-[#e8f0d8]/75'
              }`
            }
          >
            {entree.libelle}
          </NavLink>
        ))}
        <NavLink to="/refonte" className="mt-auto px-[22px] pt-6 text-sm text-[#e8f0d8]/60 no-underline">
          ← Espace adhérent
        </NavLink>
      </nav>

      <div className="grow overflow-hidden">{children}</div>
    </div>
  )
}
