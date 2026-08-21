/**
 * Fiche d'un producteur.
 *
 * Elle porte ce qui donne envie d'ouvrir son catalogue — d'où il vient, ce
 * qu'il cultive — et non les champs de gestion. La localité est mise en avant
 * parce que la proximité est ce qui distingue une AMAP d'une épicerie.
 */
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { fetchVendor } from '../../api/vendors'

export function ProducteurRefonte() {
  const { vendorId } = useParams()
  const navigate = useNavigate()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['vendor', vendorId],
    queryFn: () => fetchVendor(Number(vendorId)),
    enabled: Boolean(vendorId),
  })

  if (isLoading) return <Message>Chargement…</Message>
  if (isError || !data) return <Message>Ce producteur est introuvable.</Message>

  return (
    <div className="flex flex-col text-ink">
      <header className="flex flex-col gap-4 bg-surface px-5 pb-6 pt-4">
        <button type="button" onClick={() => navigate(-1)} aria-label="Revenir" className="w-fit bg-transparent p-0">
          <svg viewBox="0 0 24 24" className="size-[22px] fill-none stroke-ink" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
            <path d="M15 6l-6 6 6 6" />
          </svg>
        </button>

        <div className="flex items-center gap-4">
          <span className="flex size-[74px] shrink-0 items-center justify-center rounded-full border-[1.5px] border-surface-deep bg-card">
            <svg viewBox="0 0 40 40" className="size-10 fill-none stroke-control" strokeWidth={2} strokeLinecap="round">
              <path d="M8 30c0-9 5-15 12-15s12 6 12 15" />
              <path d="M20 15c-3-8 1-14 7-16 1 7-2 13-7 16z" />
            </svg>
          </span>
          <span className="flex flex-col gap-1">
            <h1 className="m-0 font-display text-[26px] leading-tight">{data.name}</h1>
            {data.city && <span className="text-[15px] text-control">{data.city}</span>}
          </span>
        </div>
      </header>

      <section className="flex flex-col gap-4 px-5 py-5">
        {data.description && (
          <p className="m-0 text-base leading-relaxed" style={{ textWrap: 'pretty' }}>
            {data.description}
          </p>
        )}

        <div className="flex gap-5 border-y border-line py-4">
          <Chiffre valeur={String(data.nbProducts)} libelle="produits" />
          {data.organic && <Chiffre valeur="Bio" libelle="certifié" />}
        </div>

        <div className="flex items-center gap-3">
          <h2 className="m-0 font-display text-lg italic">Ses produits</h2>
          <span className="h-px grow bg-line" />
        </div>

        <ul className="m-0 flex list-none flex-col p-0">
          {data.products.map((produit) => (
            <li key={produit.id} className="flex items-center justify-between gap-3 border-b border-line py-3 last:border-b-0">
              <span className="flex flex-col">
                <span className="text-base">{produit.name}</span>
                {produit.unit && <span className="text-[13px] text-ink-muted">{produit.unit}</span>}
              </span>
              <span className="font-display text-[17px]">{euros(produit.price)}</span>
            </li>
          ))}
        </ul>

        {data.products.length === 0 && (
          <p className="m-0 text-ink-muted">Aucun produit proposé en ce moment.</p>
        )}
      </section>
    </div>
  )
}

function Chiffre({ valeur, libelle }: { valeur: string; libelle: string }) {
  return (
    <span className="flex flex-col gap-0.5">
      <span className="font-display text-[22px]">{valeur}</span>
      <span className="text-[13px] text-ink-muted">{libelle}</span>
    </span>
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center bg-canvas px-8 text-center text-ink-muted">{children}</div>
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}
