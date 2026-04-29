import { useEffect } from "react";
import type { MultiDistribView } from "../../api/home";

/**
 * Modale détail d'une commande personnelle pour un MultiDistrib donné.
 * Port de templates/home.html (modal-myOrder) : tableau Qté/Produit/P.U/
 * Sous-total/Frais/Total + footer avec lien facture et total. Données déjà
 * fournies par /api/home dans md.userOrders.
 */
export function OrderDetailModal({
  md,
  onClose,
}: {
  md: MultiDistribView;
  onClose: () => void;
}) {
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

  const orders = md.userOrders ?? [];

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={`Commande du ${md.dayLabelFull}`}
      onClick={onClose}
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto"
      style={{ backgroundColor: "rgba(0,0,0,0.4)", padding: "40px 16px" }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="bg-white rounded-md w-full max-w-3xl shadow-2xl overflow-hidden"
      >
        <div className="flex items-start justify-between gap-3 px-7 pt-6 pb-4">
          <h4 className="italic text-2xl m-0 text-gray-800">
            Commande du {md.dayLabelFull}
          </h4>
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
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-gray-400 border-b border-gray-200">
                <th className="text-left font-normal py-3 pl-7">Qté</th>
                <th className="text-left font-normal py-3">Produit</th>
                <th className="text-right font-normal py-3">P.U</th>
                <th className="text-right font-normal py-3">Sous-total</th>
                <th className="text-right font-normal py-3">Frais</th>
                <th className="text-right font-normal py-3 pr-7">Total</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((o, i) => (
                <tr key={i} className="border-b border-gray-100">
                  <td className="py-2 pl-7">{o.smartQty}</td>
                  <td className="py-2 text-ac-green-dark">{o.productName}</td>
                  <td className="py-2 text-right">{Math.round(o.unitPrice)} €</td>
                  <td className="py-2 text-right">{Math.round(o.subTotal)} €</td>
                  <td className="py-2 text-right text-gray-400">
                    {o.fees > 0 ? `${o.fees.toFixed(2)} €` : ""}
                  </td>
                  <td className="py-2 text-right pr-7">{Math.round(o.total)} €</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr className="bg-gray-50 border-t border-gray-200">
                <td className="py-3 pl-7" colSpan={1}>
                  <a
                    href={`/member/invoice/${md.id}`}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 px-3 py-1.5 rounded border border-gray-300 text-sm text-gray-700 hover:bg-white"
                  >
                    <i className="icon-print" aria-hidden="true" />
                    Imprimer ma commande
                  </a>
                </td>
                <td colSpan={4} className="py-3 text-right text-gray-400 tracking-wider uppercase text-xs">
                  Total
                </td>
                <td className="py-3 text-right font-bold pr-7">
                  {Math.round(md.userOrderTotal)} €
                </td>
              </tr>
            </tfoot>
          </table>
        </div>
        <div className="flex justify-end px-7 py-4">
          <button
            type="button"
            onClick={onClose}
            className="inline-flex items-center gap-1 px-4 py-2 rounded border border-gray-300 text-sm font-bold text-gray-700 hover:bg-gray-50"
          >
            <i className="icon-delete" aria-hidden="true" />
            Fermer
          </button>
        </div>
      </div>
    </div>
  );
}
