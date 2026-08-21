/**
 * L'accueil du parcours repensé, branché sur les données du groupe.
 *
 * Il choisit lui-même la distribution à mettre en avant : celle qui est ouverte
 * aux commandes, à défaut la prochaine annoncée. C'est ce choix, fait ici plutôt
 * que laissé à l'adhérent, qui distingue ce parcours de l'accueil actuel.
 */
import { useQuery } from '@tanstack/react-query'
import { fetchHome, type MultiDistribView } from '../../api/home'
import { AccueilRefonte } from './AccueilRefonte'
import { tempsRestant } from './tempsRestant'
import { useAuthStore } from '../../store/auth'

export function AccueilConnecte() {
  const utilisateur = useAuthStore((s) => s.user)
  const { data, isLoading, isError } = useQuery({
    queryKey: ['home', 0],
    queryFn: () => fetchHome(0),
  })

  if (isLoading) return <Message>Chargement…</Message>
  if (isError || !data) return <Message>Les distributions n'ont pas pu être chargées.</Message>

  const distribution = distributionAMettreEnAvant(data.multiDistribs)
  if (!distribution) {
    return <Message>Aucune distribution n'est prévue pour le moment.</Message>
  }

  const delai = tempsRestant(distribution.orderEndAt)

  return (
    <AccueilRefonte
      prenom={utilisateur?.firstname ?? ''}
      groupe={data.groupName}
      distribution={{
        jourSemaine: distribution.dayOfWeek,
        jour: Number(distribution.day),
        mois: distribution.month,
        heureDebut: distribution.startHour,
        heureFin: distribution.endHour,
        lieu: distribution.placeAddress
          ? `${distribution.place}, ${distribution.placeAddress}`
          : distribution.place,
        // Sans délai calculable — commandes fermées, ou date absente — on
        // retombe sur la date telle que le serveur la met en forme, plutôt que
        // d'afficher un compte à rebours faux.
        tempsRestant: delai ?? distribution.orderEndDate ?? '',
        nbProducteurs: 0,
        nbProduits: distribution.productImages?.length ?? 0,
      }}
      // L'API /home n'expose pas encore les producteurs d'une distribution ;
      // la section reste masquée tant que ce n'est pas le cas.
      producteurs={[]}
      onCommander={() => {
        window.location.href = `/shop/${distribution.id}`
      }}
    />
  )
}

/**
 * Celle qui est ouverte aux commandes prime : c'est la seule sur laquelle
 * l'adhérent peut agir. À défaut, la prochaine à venir, pour qu'il sache quand
 * revenir. Les distributions passées ne sont jamais mises en avant.
 */
function distributionAMettreEnAvant(distributions: MultiDistribView[]): MultiDistribView | undefined {
  return (
    distributions.find((d) => d.canOrder) ??
    distributions.find((d) => !d.past)
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas px-8 text-center text-ink-muted">
      {children}
    </div>
  )
}
