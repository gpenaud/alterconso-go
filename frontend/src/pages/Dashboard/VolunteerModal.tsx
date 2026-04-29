import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { registerVolunteer } from "../../api/volunteers";

interface Props {
  multiDistribId: number;
  /** Rôles encore à pourvoir (libellés VolunteerRole), retournés par /api/home. */
  roles: string[];
  onClose: () => void;
}

/**
 * Modale d'inscription bénévole pour une distribution. Liste les rôles à
 * pourvoir, un clic envoie un POST /api/distributions/:id/volunteers, ferme
 * la modale et invalide ["home"] pour rafraîchir l'alerte de la card.
 */
export function VolunteerModal({ multiDistribId, roles, onClose }: Props) {
  const queryClient = useQueryClient();
  const [submitting, setSubmitting] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [onClose]);

  // Dédoublonne les rôles (l'alerte legacy peut les répéter).
  const uniqueRoles = Array.from(new Set(roles));

  const join = async (role: string) => {
    setSubmitting(role);
    setError(null);
    try {
      await registerVolunteer(multiDistribId, role);
      queryClient.invalidateQueries({ queryKey: ["home"] });
      onClose();
    } catch (e) {
      setError((e as Error).message ?? "Erreur lors de l'inscription");
    } finally {
      setSubmitting(null);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Inscription bénévole"
      onClick={onClose}
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto"
      style={{ backgroundColor: "rgba(0,0,0,0.4)", padding: "40px 16px" }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="bg-white rounded-md w-full max-w-md shadow-2xl overflow-hidden"
      >
        <div className="flex items-start justify-between gap-3 px-6 pt-5 pb-3">
          <h4 className="italic text-xl m-0 text-gray-800">Devenir bénévole</h4>
          <button
            type="button"
            onClick={onClose}
            aria-label="Fermer"
            className="text-gray-500 hover:text-gray-700 text-2xl leading-none px-1"
          >
            ×
          </button>
        </div>
        <hr className="m-0 border-gray-200" />
        <div className="px-6 py-4">
          <p className="text-sm text-gray-600 mb-3">
            Choisis le rôle que tu souhaites prendre :
          </p>
          <ul className="space-y-2">
            {uniqueRoles.map((role) => (
              <li
                key={role}
                className="flex items-center justify-between gap-3 border border-gray-200 rounded-md px-3 py-2"
              >
                <span className="text-sm text-gray-800">{role}</span>
                <button
                  type="button"
                  onClick={() => join(role)}
                  disabled={submitting !== null}
                  className="px-3 py-1.5 rounded-md text-sm font-medium bg-ac-green hover:bg-ac-green-dark text-white disabled:opacity-60"
                >
                  {submitting === role ? "Envoi…" : "S'inscrire"}
                </button>
              </li>
            ))}
          </ul>
          {error && (
            <p className="text-red-600 text-sm mt-3">{error}</p>
          )}
        </div>
      </div>
    </div>
  );
}
