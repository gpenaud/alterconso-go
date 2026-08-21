/**
 * Catalogues et produits.
 *
 * Trois colonnes : la navigation, la liste des catalogues, le contenu de celui
 * qu'on regarde. La liste du milieu porte l'état de chacun — c'est la réponse
 * au coup d'œil « qu'est-ce qui est ouvert en ce moment ».
 */
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getCatalogs, type Catalog } from '../../api/catalogs'
import { fetchCatalogProducts } from '../../api/adminCatalogs'
import { useAuthStore } from '../../store/auth'

export function AdminCatalogues() {
  const groupId = useAuthStore((s) => s.currentGroupId)
  const [choisi, setChoisi] = useState<number | null>(null)

  const { data: catalogues, isLoading } = useQuery({
    queryKey: ['catalogs', groupId],
    queryFn: () => getCatalogs(groupId!),
    enabled: Boolean(groupId),
  })

  const catalogueCourant = choisi ?? catalogues?.[0]?.id ?? null

  const { data: detail } = useQuery({
    queryKey: ['catalog-products', catalogueCourant],
    queryFn: () => fetchCatalogProducts(catalogueCourant!),
    enabled: Boolean(catalogueCourant),
  })

  if (isLoading) return <Message>Chargement…</Message>
  if (!catalogues || catalogues.length === 0) return <Message>Aucun catalogue dans ce groupe.</Message>

  return (
    <div className="flex h-screen">
      <aside className="flex w-[300px] shrink-0 flex-col overflow-y-auto border-r-[1.5px] border-line bg-tint/40">
        <h2 className="m-0 px-5 py-4 font-display text-lg italic">
          {catalogues.length} catalogue{catalogues.length > 1 ? 's' : ''}
        </h2>
        {catalogues.map((catalogue) => (
          <button
            key={catalogue.id}
            type="button"
            onClick={() => setChoisi(catalogue.id)}
            className={`flex flex-col gap-1 border-b border-line px-[18px] py-3.5 text-left ${
              catalogue.id === catalogueCourant ? 'border-l-[3px] border-l-action bg-card' : 'bg-transparent'
            }`}
          >
            <span className="text-base font-semibold">{catalogue.name}</span>
            <span className="text-[13px] text-ink-muted">{periode(catalogue)}</span>
          </button>
        ))}
      </aside>

      <div className="flex grow flex-col overflow-y-auto">
        <header className="flex items-start justify-between gap-6 bg-surface px-8 py-4">
          <div className="flex flex-col gap-1">
            <h1 className="m-0 font-display text-2xl">{detail?.name ?? '…'}</h1>
            {detail?.vendorName && <p className="m-0 text-sm text-surface-deep">{detail.vendorName}</p>}
          </div>
          <a
            href={`/contractAdmin/products/${catalogueCourant}`}
            className="min-h-[44px] rounded-control bg-action px-5 text-[15px] font-semibold leading-[44px] text-card no-underline"
          >
            Gérer les produits
          </a>
        </header>

        <div className="p-8">
          <div className="overflow-hidden rounded-card border-[1.5px] border-line bg-card">
            <div className="grid grid-cols-[90px_2fr_1fr_110px_110px] gap-3 border-b-[1.5px] border-line bg-tint/60 px-5 py-3 text-xs uppercase tracking-wider text-ink-muted">
              <span>Référence</span>
              <span>Nom</span>
              <span>Conditionnement</span>
              <span className="text-right">Prix</span>
              <span className="text-right">Stock</span>
            </div>

            {(detail?.products ?? []).map((produit) => (
              <div
                key={produit.id}
                className={`grid grid-cols-[90px_2fr_1fr_110px_110px] items-center gap-3 border-b border-line px-5 py-3 last:border-b-0 ${
                  produit.active ? '' : 'bg-tint/20 text-ink-faint'
                }`}
              >
                <span className="text-sm text-ink-muted">{produit.ref || '—'}</span>
                <span className="flex items-center gap-2 text-base">
                  {produit.name}
                  {produit.needsWeighing && (
                    <span className="rounded-[3px] bg-action-soft px-2 py-0.5 text-xs text-action-ink">à peser</span>
                  )}
                  {!produit.active && (
                    <span className="rounded-[3px] border border-line px-2 py-0.5 text-xs text-ink-muted">retiré</span>
                  )}
                </span>
                <span className="text-sm text-ink-muted">{produit.unit}</span>
                <span className="text-right font-display text-base">{euros(produit.price)}</span>
                <span className="text-right text-sm">{stock(produit.stockTracked, produit.stock)}</span>
              </div>
            ))}

            {detail?.products.length === 0 && (
              <p className="m-0 px-5 py-6 text-ink-muted">Ce catalogue n'a aucun produit.</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

/** Un stock non suivi n'est pas un stock à zéro : le dire évite de croire à
 *  une rupture. */
function stock(suivi: boolean, valeur?: number) {
  if (!suivi) return <span className="text-ink-muted">illimité</span>
  if (valeur == null || valeur <= 0) return <span className="text-action-ink">épuisé</span>
  if (valeur <= 10) return <span className="text-action-ink">{valeur}</span>
  return <span>{valeur}</span>
}

function periode(catalogue: Catalog) {
  if (!catalogue.startDate && !catalogue.endDate) return 'Sans période définie'
  const debut = catalogue.startDate ? new Date(catalogue.startDate).toLocaleDateString('fr-FR') : '…'
  const fin = catalogue.endDate ? new Date(catalogue.endDate).toLocaleDateString('fr-FR') : '…'
  return `Du ${debut} au ${fin}`
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center px-8 text-center text-ink-muted">{children}</div>
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}
