import { useState } from "react";
import { Link } from "react-router-dom";
import type { MultiDistribView } from "../../api/home";
import { OrderDetailModal } from "./OrderDetailModal";

/**
 * Carte d'une distribution sur la page d'accueil. Port du legacy
 * templates/home.html : bandeau date/lieu, vignettes produits, CTA
 * "Commander" selon l'état d'ouverture, alerte bénévolat. Le détail des
 * commandes du membre + facture restent à porter (modale + footer).
 */
export function MultiDistribCard({ md }: { md: MultiDistribView }) {
  const headerBg = md.canOrder ? "bg-ac-green" : "bg-white";
  const headerText = md.canOrder ? "text-white" : "text-gray-700";
  const [orderOpen, setOrderOpen] = useState(false);

  return (
    <div className="bg-white rounded-lg border border-gray-200 shadow-sm overflow-hidden">
      {/* Header bar */}
      <div className={`flex items-center gap-4 px-4 py-3 ${headerBg} ${headerText}`}>
        <div className="flex-shrink-0 bg-white text-gray-700 rounded-md text-center px-3 py-1 border border-gray-200">
          <div className="text-xs leading-tight">{md.dayOfWeek}</div>
          <div className="text-2xl font-bold leading-none text-red-700">{md.day}</div>
          <div className="text-xs leading-tight">{md.month}</div>
        </div>
        <div className="flex-1 min-w-0 text-sm leading-snug">
          <div className="flex items-center gap-1">
            <i className="icon-clock" aria-hidden="true" />
            <span>
              Distribution de {md.startHour} à {md.endHour}
            </span>
          </div>
          <div className="flex items-center gap-1">
            <i className="icon-map-marker" aria-hidden="true" />
            <span>{md.place}</span>
          </div>
          {md.placeAddress && (
            <div className={`text-xs ${md.canOrder ? "text-white/90" : "text-gray-500"}`}>
              {md.placeAddress}
            </div>
          )}
        </div>
      </div>

      {/* Vignettes produits */}
      {md.productImages && md.productImages.length > 0 && (md.canOrder || md.orderNotYetOpen) && (
        <div className="flex gap-1 px-3 py-2 justify-center overflow-hidden">
          {md.productImages.map((img, i) => (
            <div
              key={i}
              title={img.name}
              className="w-16 h-16 rounded-md border border-gray-200 bg-white overflow-hidden flex-shrink-0"
            >
              <img src={img.url} alt="" className="w-full h-full object-cover" />
            </div>
          ))}
        </div>
      )}

      {/* Ma commande */}
      {md.userOrders && md.userOrders.length > 0 && (
        <div className="text-center px-4 py-2">
          <button
            type="button"
            onClick={() => setOrderOpen(true)}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium border border-gray-300 text-gray-700 bg-white hover:bg-gray-50"
          >
            <i className="icon-basket" aria-hidden="true" />
            Ma commande : {Math.round(md.userOrderTotal)} €
          </button>
        </div>
      )}

      {/* État de commande */}
      {md.distributions && (
        <div className="text-center px-4 py-3">
          {md.canOrder ? (
            <>
              <Link
                to={`/shop/${md.id}`}
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-md text-base font-semibold bg-ac-green hover:bg-ac-green-dark text-white transition-colors"
              >
                <i className="icon-chevron-right" aria-hidden="true" />
                Commander
              </Link>
              {md.orderEndDate && (
                <div className="text-gray-500 text-sm mt-3">
                  <i className="icon-clock" aria-hidden="true" /> La commande fermera le {md.orderEndDate}
                </div>
              )}
            </>
          ) : md.orderNotYetOpen ? (
            <span className="text-gray-400 text-base block py-2">
              <i className="icon-clock" aria-hidden="true" /> La commande ouvrira {md.orderStartDate}
            </span>
          ) : (
            <span className="text-gray-400 text-sm">
              <i className="icon-clock" aria-hidden="true" /> Commandes fermées
            </span>
          )}
        </div>
      )}

      {/* Bénévolat */}
      {md.volunteerNeeded > 0 && (
        <div className="px-4 py-3">
          <div className="bg-red-50 border border-red-200 rounded-md p-4 text-center">
            <p className="mb-2 text-sm">
              <i className="icon-alert text-red-700" aria-hidden="true" /> Nous avons besoin de{" "}
              <b>{md.volunteerNeeded}</b> bénévole(s) pour les rôles suivants :
            </p>
            <p className="text-red-700 font-bold text-sm mb-3">
              {md.volunteerRoles?.join(", ")}
            </p>
            <a
              href="/distribution/volunteersCalendar"
              className="inline-flex items-center gap-1 px-3 py-1.5 rounded-md text-sm bg-red-600 hover:bg-red-700 text-white"
            >
              <i className="icon-chevron-right" aria-hidden="true" />
              Inscription
            </a>
          </div>
        </div>
      )}

      {/* Footer facture (distributions passées avec commande) */}
      {md.past && md.userOrders && md.userOrders.length > 0 && (
        <div className="px-4 py-3 border-t border-gray-100">
          <a
            href={`/member/invoice/${md.id}`}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-md text-sm border border-gray-300 text-gray-700 hover:bg-gray-50"
          >
            <i className="icon-print" aria-hidden="true" />
            Ma facture
          </a>
        </div>
      )}

      {orderOpen && <OrderDetailModal md={md} onClose={() => setOrderOpen(false)} />}
    </div>
  );
}
