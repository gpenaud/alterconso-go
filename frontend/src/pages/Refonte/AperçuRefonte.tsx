/**
 * Page d'aperçu de la refonte, servie sur /refonte.
 *
 * Elle existe pour juger la direction sur un vrai navigateur, avec les vraies
 * polices, avant de brancher quoi que ce soit sur l'API. Les données sont donc
 * fixes et assumées comme telles — elles ne viennent d'aucun groupe réel.
 *
 * À supprimer quand l'écran passera en production.
 */
import { AccueilRefonte } from './AccueilRefonte'

export function AperçuRefonte() {
  return (
    <AccueilRefonte
      prenom="Guillaume"
      groupe="Val de Brenne"
      distribution={{
        jourSemaine: 'jeudi',
        jour: 27,
        mois: 'août',
        heureDebut: '18h00',
        heureFin: '19h30',
        lieu: 'Salle des fêtes, Neuvy',
        tempsRestant: '2 jours',
        nbProducteurs: 7,
        nbProduits: 84,
      }}
      producteurs={[
        { id: 1, nom: 'Ferme du Jointout', ville: 'Neuvy-sur-Barangeon', bio: true, embleme: 'legume' },
        { id: 2, nom: 'Articholoko', ville: 'Vouzeron', bio: true, embleme: 'oeuf' },
        { id: 3, nom: 'Aux Retrouvailles', ville: 'Nançay', embleme: 'fromage' },
      ]}
      onCommander={() => {}}
      autresDistributions={2}
      onVoirAutres={() => {}}
    />
  )
}
