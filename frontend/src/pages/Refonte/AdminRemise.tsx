/**
 * Remise des paniers, le jour de la distribution.
 *
 * Le seul écran d'administration pensé pour le téléphone : on le tient debout,
 * une main occupée, souvent dans une salle mal éclairée. D'où les grandes
 * zones tactiles, une ligne par adhérent, et le décompte de ce qui reste.
 *
 * Cocher marque le panier remis côté serveur : un téléphone qui se verrouille
 * ou un second bénévole sur sa propre tablette ne doivent pas repartir de zéro.
 */
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { fetchHome } from '../../api/home'
import { fetchDistributionOrders, markDelivered, type AdminOrderLine } from '../../api/adminOrders'

export function AdminRemise() {
  const queryClient = useQueryClient()
  const [recherche, setRecherche] = useState('')

  const { data: accueil } = useQuery({ queryKey: ['home', 0], queryFn: () => fetchHome(0) })
  const distribution = accueil?.multiDistribs.find((d) => d.active) ?? accueil?.multiDistribs.find((d) => !d.past)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-orders', distribution?.id],
    queryFn: () => fetchDistributionOrders(distribution!.id),
    enabled: Boolean(distribution),
  })

  const remise = useMutation({
    mutationFn: ({ userId, delivered }: { userId: number; delivered: boolean }) =>
      markDelivered(distribution!.id, userId, delivered),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin-orders', distribution?.id] }),
  })

  const paniers = useMemo(() => parAdherent(data?.lines ?? []), [data])
  const filtres = paniers.filter((p) => p.nom.toLowerCase().includes(recherche.toLowerCase()))
  const remis = paniers.filter((p) => p.remis).length

  if (!distribution) return <Message>Aucune distribution en cours.</Message>
  if (isLoading) return <Message>Chargement…</Message>

  return (
    <div className="flex min-h-screen flex-col bg-canvas text-ink">
      <header className="bg-surface px-4 py-4">
        <div className="flex items-center justify-between">
          <h1 className="m-0 font-display text-[22px]">Remise des paniers</h1>
          <span className="text-sm text-surface-deep">
            {distribution.day} {distribution.month}
          </span>
        </div>
        <div className="mt-3 flex items-center gap-2.5">
          <span className="h-2.5 grow overflow-hidden rounded-full bg-surface-deep/20">
            <span
              className="block h-full bg-surface-deep"
              style={{ width: `${paniers.length > 0 ? Math.round((remis / paniers.length) * 100) : 0}%` }}
            />
          </span>
          <span className="font-display text-[17px]">
            {remis} / {paniers.length}
          </span>
        </div>
      </header>

      <label className="flex items-center gap-2.5 border-b-[1.5px] border-line bg-card px-4 py-3">
        <svg viewBox="0 0 24 24" className="size-[18px] shrink-0 fill-none stroke-ink-muted" strokeWidth={2} strokeLinecap="round">
          <circle cx="11" cy="11" r="6.5" />
          <path d="M16 16l4 4" />
        </svg>
        <input
          type="search"
          value={recherche}
          placeholder="Chercher un adhérent"
          onChange={(e) => setRecherche(e.target.value)}
          className="w-full border-0 bg-transparent text-base outline-none"
        />
      </label>

      <ul className="m-0 grow list-none p-0">
        {filtres.map((panier) => (
          <li key={panier.userId}>
            <button
              type="button"
              onClick={() => remise.mutate({ userId: panier.userId, delivered: !panier.remis })}
              className={`flex w-full items-center gap-3.5 border-b border-line px-4 py-4 text-left ${
                panier.remis ? 'bg-tint/40' : 'bg-transparent'
              }`}
            >
              <span
                className={`flex size-[30px] shrink-0 items-center justify-center rounded-control border-[1.5px] ${
                  panier.remis ? 'border-control bg-control' : 'border-ink/35'
                }`}
              >
                {panier.remis && (
                  <svg viewBox="0 0 24 24" className="size-[17px] fill-none stroke-card" strokeWidth={3.2} strokeLinecap="round" strokeLinejoin="round">
                    <path d="M5 13l4 4L19 7" />
                  </svg>
                )}
              </span>

              <span className="flex grow flex-col gap-0.5">
                <span className={`text-[17px] ${panier.remis ? 'text-ink-muted line-through' : ''}`}>{panier.nom}</span>
                <span className="text-sm text-ink-muted">{panier.resume}</span>
                {panier.aPeser && !panier.remis && (
                  <span className="text-[13px] text-action-ink">à peser</span>
                )}
              </span>

              <span className={`font-display text-base ${panier.remis ? 'text-ink-faint' : ''}`}>{euros(panier.total)}</span>
            </button>
          </li>
        ))}

        {filtres.length === 0 && <li className="px-4 py-6 text-center text-ink-muted">Personne ne correspond.</li>}
      </ul>

      <footer className="flex items-center justify-between gap-4 border-t-[1.5px] border-line bg-card px-4 pb-5 pt-3">
        <span className="flex flex-col gap-0.5">
          <span className="text-[13px] text-ink-muted">Reste à remettre</span>
          <span className="font-display text-xl">
            {paniers.length - remis} panier{paniers.length - remis > 1 ? 's' : ''}
          </span>
        </span>
        <a
          href={`/distribution/list/${distribution.id}`}
          className="min-h-[50px] rounded-control border-[1.5px] border-ink/40 px-5 text-base leading-[50px] text-ink no-underline"
        >
          Feuille complète
        </a>
      </footer>
    </div>
  )
}

interface Panier {
  userId: number
  nom: string
  resume: string
  total: number
  remis: boolean
  aPeser: boolean
}

/** Un panier, c'est un adhérent — pas une ligne de commande. */
function parAdherent(lignes: AdminOrderLine[]): Panier[] {
  const parUtilisateur = new Map<number, Panier & { produits: string[] }>()
  for (const ligne of lignes) {
    const panier =
      parUtilisateur.get(ligne.userId) ??
      { userId: ligne.userId, nom: ligne.userName, resume: '', total: 0, remis: ligne.delivered, aPeser: false, produits: [] }
    panier.total += ligne.total
    panier.aPeser = panier.aPeser || (ligne.needsWeighing && !ligne.weighed)
    // Le drapeau porte sur toutes les lignes ensemble : une seule non remise
    // suffit à considérer le panier comme non remis.
    panier.remis = panier.remis && ligne.delivered
    panier.produits.push(`${arrondi(ligne.quantity)} ${ligne.product}`)
    parUtilisateur.set(ligne.userId, panier)
  }
  return [...parUtilisateur.values()]
    .map((p) => ({ ...p, resume: p.produits.join(' · ') }))
    .sort((a, b) => a.nom.localeCompare(b.nom, 'fr'))
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center bg-canvas px-8 text-center text-ink-muted">{children}</div>
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}

function arrondi(quantite: number) {
  return Number.isInteger(quantite) ? String(quantite) : quantite.toFixed(1)
}
