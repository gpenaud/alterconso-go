import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { fetchHome } from '../../api/home'
import { DistributionsRefonte, type DistributionListee } from './DistributionsRefonte'
import { tempsRestant } from './tempsRestant'

export function DistributionsConnecte() {
  const navigate = useNavigate()
  const { data, isLoading } = useQuery({ queryKey: ['home', 0], queryFn: () => fetchHome(0) })

  if (isLoading || !data) {
    return <div className="flex min-h-screen items-center justify-center bg-canvas text-ink-muted">Chargement…</div>
  }

  const distributions: DistributionListee[] = data.multiDistribs.map((d) => ({
    id: d.id,
    jourSemaine: d.dayOfWeek,
    jour: Number(d.day),
    mois: d.month,
    heureDebut: d.startHour,
    heureFin: d.endHour,
    lieu: d.place,
    etat: etatLisible(d),
    ouverte: d.canOrder,
    passee: d.past,
    montantCommande: d.userOrderTotal > 0 ? d.userOrderTotal : undefined,
  }))

  return (
    <DistributionsRefonte
      distributions={distributions}
      onChoisir={(id) => {
        const choisie = data.multiDistribs.find((d) => d.id === id)
        // Une distribution fermée n'ouvre pas la boutique : on renvoie sur
        // l'accueil plutôt que sur un catalogue qui refusera toute commande.
        if (choisie?.canOrder) navigate(`/shop/${id}`)
        else navigate('/refonte')
      }}
      onRetour={() => navigate('/refonte')}
    />
  )
}

/** Une phrase par état, plutôt qu'un statut technique à décoder. */
function etatLisible(d: { canOrder: boolean; past: boolean; orderEndAt?: string; orderStartDate?: string; orderEndDate?: string }): string {
  if (d.past) return 'Distribution passée'
  if (d.canOrder) {
    const delai = tempsRestant(d.orderEndAt)
    return delai ? `Encore ${delai} pour commander` : `Clôture le ${d.orderEndDate ?? ''}`
  }
  return d.orderStartDate ? `Ouvre le ${d.orderStartDate}` : 'Commandes fermées'
}
