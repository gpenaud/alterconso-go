/**
 * Met en mots le délai avant la clôture des commandes.
 *
 * En mots plutôt qu'en compte à rebours qui défile : l'urgence se dit, elle n'a
 * pas besoin de clignoter. Et un adhérent qui lit « encore 2 jours » agit ;
 * devant « 47:12:08 », il regarde.
 *
 * Retourne null quand la date est absente ou dépassée — l'appelant décide alors
 * quoi afficher, plutôt que de recevoir un « 0 jour » trompeur.
 */
export function tempsRestant(dateISO: string | undefined, maintenant = new Date()): string | null {
  if (!dateISO) return null

  const fin = new Date(dateISO)
  if (Number.isNaN(fin.getTime())) return null

  const minutes = Math.floor((fin.getTime() - maintenant.getTime()) / 60000)
  if (minutes <= 0) return null

  const jours = Math.floor(minutes / 1440)
  if (jours >= 1) return jours === 1 ? '1 jour' : `${jours} jours`

  const heures = Math.floor(minutes / 60)
  if (heures >= 1) return heures === 1 ? '1 heure' : `${heures} heures`

  return minutes === 1 ? '1 minute' : `${minutes} minutes`
}
