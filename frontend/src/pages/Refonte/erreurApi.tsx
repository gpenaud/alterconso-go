/**
 * Met en mots ce qu'une requête refusée veut dire, et propose la sortie.
 *
 * Un écran qui dit seulement « ça n'a pas pu être chargé » laisse chercher :
 * session expirée ? droits manquants ? serveur éteint ? Le serveur, lui, le
 * sait — il faut reprendre ce qu'il dit plutôt que de tout confondre.
 */
export interface ErreurLisible {
  message: string
  /** Ce que le lecteur peut faire, quand il y a quelque chose à faire. */
  action?: { libelle: string; href: string }
}

export function lireErreur(error: unknown, sujet: string): ErreurLisible {
  const reponse = (error as { response?: { status?: number; data?: { error?: string } } })?.response
  const statut = reponse?.status
  const detail = reponse?.data?.error ?? ''

  // Le cas le plus fréquent en développement, et le plus vite corrigé : le
  // jeton ne porte aucun groupe courant.
  if (statut === 400 && detail.includes('no current group')) {
    return {
      message: 'Aucun groupe sélectionné pour ce compte.',
      action: { libelle: 'Choisir un groupe', href: '/user/choose' },
    }
  }

  if (statut === 401) {
    return {
      message: 'Votre session a expiré.',
      action: { libelle: 'Se reconnecter', href: '/user/login' },
    }
  }

  if (statut === 403) {
    return { message: `Vous n'avez pas les droits nécessaires pour consulter ${sujet}.` }
  }

  if (statut === undefined) {
    return { message: `Le serveur n'a pas répondu. Est-il démarré ?` }
  }

  return { message: `${sujet} n'a pas pu être chargé (erreur ${statut}).` }
}

export function EcranErreur({ erreur }: { erreur: ErreurLisible }) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-canvas px-8 text-center">
      <p className="m-0 text-base text-ink-muted">{erreur.message}</p>
      {erreur.action && (
        <a
          href={erreur.action.href}
          className="min-h-[48px] rounded-control bg-control px-6 text-base font-semibold leading-[48px] text-card no-underline"
        >
          {erreur.action.libelle}
        </a>
      )}
    </div>
  )
}
