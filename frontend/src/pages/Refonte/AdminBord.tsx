/**
 * Tableau de bord de la prochaine distribution.
 *
 * Organisé autour d'une seule question : que reste-t-il à faire avant la
 * clôture ? Les compteurs sont là pour y répondre, pas pour meubler — celui des
 * bénévoles manquants est le seul qui alerte, parce que c'est le seul qui
 * bloque vraiment un jeudi.
 */
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { fetchHome, type MultiDistribView } from '../../api/home'
import { fetchDistributionSummary } from '../../api/adminSummary'
import { tempsRestant } from './tempsRestant'

export function AdminBord() {
  const navigate = useNavigate()
  const { data: accueil, isLoading } = useQuery({ queryKey: ['home', 0], queryFn: () => fetchHome(0) })

  const distribution = accueil?.multiDistribs.find((d) => d.canOrder) ?? accueil?.multiDistribs.find((d) => !d.past)

  const { data: resume } = useQuery({
    queryKey: ['admin-summary', distribution?.id],
    queryFn: () => fetchDistributionSummary(distribution!.id),
    enabled: Boolean(distribution),
  })

  if (isLoading) return <Message>Chargement…</Message>
  if (!distribution) return <Message>Aucune distribution n'est programmée.</Message>

  const delai = tempsRestant(distribution.orderEndAt)
  const sansCommande = resume ? resume.nbMembers - resume.nbOrders : 0

  return (
    <div className="flex flex-col">
      <header className="flex items-center justify-between gap-6 bg-surface px-8 py-5">
        <div className="flex flex-col gap-1">
          <h1 className="m-0 font-display text-[26px]">
            Distribution du {distribution.dayOfWeek} {distribution.day} {distribution.month}
          </h1>
          <p className="m-0 text-[15px] text-surface-deep">
            {distribution.startHour} – {distribution.endHour} · {distribution.place}
            {delai && ` · clôture dans ${delai}`}
          </p>
        </div>
        <a
          href={`/distribution/listByDate/${dateISO(distribution)}/${accueil?.groupId}/print`}
          className="min-h-[46px] rounded-control bg-action px-6 text-base font-semibold leading-[46px] text-card no-underline"
        >
          Feuille de distribution
        </a>
      </header>

      <div className="flex flex-col gap-6 p-8">
        <div className="grid grid-cols-4 gap-4">
          <Compteur titre="Commandes reçues" valeur={resume ? String(resume.nbOrders) : '—'} note={resume ? `sur ${resume.nbMembers} adhérents` : ''} />
          <Compteur titre="Montant collecté" valeur={resume ? euros(resume.total) : '—'} note={resume && resume.nbOrders > 0 ? `${euros(resume.averageOrder)} en moyenne` : ''} />
          <Compteur titre="Producteurs" valeur={resume ? String(resume.nbVendors) : '—'} note="à cette distribution" />
          <Compteur
            titre="Permanences"
            valeur={resume ? String(resume.volunteerNeeded) : '—'}
            note="bénévoles manquants"
            alerte={Boolean(resume && resume.volunteerNeeded > 0)}
          />
        </div>

        <div className="flex items-start gap-6">
          <section className="grow overflow-hidden rounded-card border-[1.5px] border-line bg-card">
            <h2 className="m-0 border-b border-line px-5 py-4 font-display text-lg italic">Ce qu'il reste à faire</h2>

            {resume && resume.volunteerNeeded > 0 && (
              <Tache
                urgente
                libelle={`Trouver ${resume.volunteerNeeded} bénévole${resume.volunteerNeeded > 1 ? 's' : ''} pour la distribution`}
                action="Relancer"
                onAction={() => navigate('/refonte/message')}
              />
            )}
            {sansCommande > 0 && (
              <Tache
                libelle={`${sansCommande} adhérents n'ont pas encore commandé`}
                action="Écrire"
                onAction={() => navigate('/refonte/message')}
              />
            )}
            {resume && resume.volunteerNeeded === 0 && sansCommande <= 0 && (
              <p className="m-0 px-5 py-5 text-ink-muted">Rien ne manque : la distribution est prête.</p>
            )}
          </section>

          <section className="w-[400px] shrink-0 overflow-hidden rounded-card border-[1.5px] border-line bg-card">
            <h2 className="m-0 border-b border-line px-5 py-4 font-display text-lg italic">Les plus commandés</h2>
            <div className="flex flex-col gap-3.5 px-5 py-4">
              {(resume?.topProducts ?? []).map((produit) => (
                <div key={produit.name} className="flex items-center gap-3">
                  <span className="w-[150px] shrink-0 truncate text-[15px]">{produit.name}</span>
                  <span className="h-2.5 grow overflow-hidden rounded-full bg-tint">
                    <span
                      className="block h-full bg-control"
                      style={{ width: `${pourcentage(produit.quantity, resume!.topProducts[0].quantity)}%` }}
                    />
                  </span>
                  <span className="w-8 text-right font-display text-base">{arrondi(produit.quantity)}</span>
                </div>
              ))}
              {(resume?.topProducts.length ?? 0) === 0 && (
                <p className="m-0 text-ink-muted">Aucune commande pour l'instant.</p>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

function Compteur({ titre, valeur, note, alerte }: { titre: string; valeur: string; note?: string; alerte?: boolean }) {
  return (
    <div className={`rounded-card border-[1.5px] p-[18px] ${alerte ? 'border-action bg-action-soft' : 'border-line bg-card'}`}>
      <p className={`m-0 text-[13px] ${alerte ? 'text-action-ink' : 'text-ink-muted'}`}>{titre}</p>
      <p className={`m-0 mt-1.5 font-display text-[38px] leading-none ${alerte ? 'text-action-ink' : ''}`}>{valeur}</p>
      {note && <p className={`m-0 mt-1 text-[13px] ${alerte ? 'text-action-ink' : 'text-ink-muted'}`}>{note}</p>}
    </div>
  )
}

function Tache({ libelle, action, onAction, urgente }: { libelle: string; action: string; onAction: () => void; urgente?: boolean }) {
  return (
    <div className="flex items-center gap-3.5 border-b border-line px-5 py-4 last:border-b-0">
      <span className={`size-6 shrink-0 rounded-control border-[1.5px] ${urgente ? 'border-action bg-action-soft' : 'border-ink/35'}`} />
      <span className="grow text-base">{libelle}</span>
      <button type="button" onClick={onAction} className="bg-transparent p-0 text-sm text-control underline">
        {action}
      </button>
    </div>
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center px-8 text-center text-ink-muted">{children}</div>
}

/* La feuille de distribution est encore servie par Go, et son adresse attend la
 * date de la distribution — pas celle de la clôture, qui pointerait sur une
 * feuille vide. */
function dateISO(d: MultiDistribView): string {
  return d.startAt ? d.startAt.slice(0, 10) : ''
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 })
}

function arrondi(quantite: number) {
  return Number.isInteger(quantite) ? String(quantite) : quantite.toFixed(1)
}

function pourcentage(valeur: number, max: number) {
  return max > 0 ? Math.round((valeur / max) * 100) : 0
}
