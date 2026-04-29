import api from "./client";

/**
 * Inscrit l'utilisateur courant comme bénévole sur le multiDistrib donné.
 * Le `role` correspond au libellé de la VolunteerRole (ex "Permanence du
 * marché"). Le backend (service.Register) gère l'unicité : un user n'est
 * inscrit qu'une fois par MultiDistrib.
 */
export function registerVolunteer(multiDistribId: number, role?: string) {
  return api
    .post<{ success: boolean; id: number }>(
      `/distributions/${multiDistribId}/volunteers`,
      role ? { role } : {},
    )
    .then((r) => r.data);
}
