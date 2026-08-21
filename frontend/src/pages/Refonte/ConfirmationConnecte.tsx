import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { fetchHome } from '../../api/home'
import { ConfirmationRefonte } from './ConfirmationRefonte'

export function ConfirmationConnecte() {
  const { multiDistribId } = useParams()
  const navigate = useNavigate()
  const { data, isLoading, isError } = useQuery({ queryKey: ['home', 0], queryFn: () => fetchHome(0) })

  if (isLoading) return <Message>Chargement…</Message>
  if (isError || !data) return <Message>Cette commande n'a pas pu être chargée.</Message>

  const distribution = data.multiDistribs.find((d) => String(d.id) === multiDistribId)
  if (!distribution) return <Message>Cette distribution est introuvable.</Message>

  const lignes = distribution.userOrders ?? []
  if (lignes.length === 0) {
    return <Message>Vous n'avez pas de commande pour cette distribution.</Message>
  }

  return (
    <ConfirmationRefonte
      jourLabel={distribution.dayLabelFull}
      heureDebut={distribution.startHour}
      heureFin={distribution.endHour}
      lieu={distribution.place}
      lignes={lignes.map((l) => ({
        produit: l.productName,
        quantite: l.smartQty,
        total: l.total,
      }))}
      total={distribution.userOrderTotal}
      // Le bouton de modification suit cette valeur : commandes closes, elle est
      // absente, et proposer de modifier serait une promesse que la boutique ne
      // tiendrait pas.
      modifiableJusquA={distribution.canOrder ? distribution.orderEndDate : undefined}
      onModifier={() => navigate(`/shop/${distribution.id}`)}
      onRetour={() => navigate('/refonte')}
    />
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas px-8 text-center text-ink-muted">
      {children}
    </div>
  )
}
