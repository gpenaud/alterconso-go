/**
 * Fiche d'un adhérent.
 *
 * À gauche ce qui est stable — coordonnées, adhésion, droits ; à droite ce qui
 * vit : ses commandes. L'état « non retiré » y figure, parce que c'est
 * l'information qu'un responsable cherche vraiment en ouvrant une fiche, et
 * qu'aucun écran ne donne aujourd'hui.
 */
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { fetchMember } from '../../api/adminMember'

export function AdminMembre() {
  const { memberId } = useParams()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['admin-member', memberId],
    queryFn: () => fetchMember(Number(memberId)),
    enabled: Boolean(memberId),
  })

  if (isLoading) return <Message>Chargement…</Message>
  if (isError || !data) return <Message>Cette fiche ne vous est pas accessible.</Message>

  return (
    <div className="flex flex-col">
      <header className="flex items-center justify-between gap-6 bg-surface px-8 py-4">
        <div className="flex items-center gap-4">
          <Link to="/refonte/admin/membres" aria-label="Revenir" className="text-ink">
            <svg viewBox="0 0 24 24" className="size-[22px] fill-none stroke-ink" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
              <path d="M15 6l-6 6 6 6" />
            </svg>
          </Link>
          <span className="flex size-[54px] items-center justify-center rounded-full border-[1.5px] border-surface-deep bg-card font-display text-xl">
            {initiales(data.name)}
          </span>
          <div className="flex flex-col gap-0.5">
            <h1 className="m-0 font-display text-[25px]">{data.name}</h1>
            <p className="m-0 text-sm text-surface-deep">
              {data.memberSince && `Adhérent depuis ${data.memberSince} · `}
              {data.orders.length} commande{data.orders.length > 1 ? 's' : ''} enregistrée
              {data.orders.length > 1 ? 's' : ''}
            </p>
          </div>
        </div>
        <a
          href={`/member/edit/${data.userId}`}
          className="min-h-[44px] rounded-control border-[1.5px] border-surface-deep bg-card px-5 text-[15px] leading-[44px] text-ink no-underline"
        >
          Modifier
        </a>
      </header>

      <div className="flex items-start gap-6 p-8">
        <aside className="flex w-[380px] shrink-0 flex-col gap-4">
          <Bloc titre="Coordonnées">
            <Champ libelle="Courriel" valeur={data.email} />
            <Champ libelle="Téléphone" valeur={data.phone || 'Non renseigné'} />
            <Champ libelle="Adresse" valeur={data.address || 'Non renseignée'} />
          </Bloc>

          <Bloc titre="Adhésion">
            <div className="flex items-center gap-3.5 px-4 py-4">
              <span
                className={`flex size-[34px] shrink-0 items-center justify-center rounded-full border-[1.5px] ${
                  data.membershipUpToDate ? 'border-control bg-tint' : 'border-action bg-action-soft'
                }`}
              >
                {data.membershipUpToDate ? (
                  <svg viewBox="0 0 24 24" className="size-[17px] fill-none stroke-control" strokeWidth={3} strokeLinecap="round" strokeLinejoin="round">
                    <path d="M5 13l4 4L19 7" />
                  </svg>
                ) : (
                  <span className="font-display text-lg text-action-ink">!</span>
                )}
              </span>
              <span className="flex flex-col gap-0.5">
                <span className="text-base">
                  {data.membershipYear === 0
                    ? 'Jamais adhéré'
                    : data.membershipUpToDate
                      ? `À jour · ${euros(data.membershipFee)}`
                      : `Expirée depuis ${data.membershipYear}`}
                </span>
                {data.membershipYear > 0 && (
                  <span className="text-sm text-ink-muted">Dernière adhésion en {data.membershipYear}</span>
                )}
              </span>
            </div>
          </Bloc>

          <Bloc titre="Droits">
            <div className="flex flex-col gap-2 px-4 py-4">
              {data.role ? (
                <span className="text-base text-control">{data.role}</span>
              ) : data.delegations.length > 0 ? (
                data.delegations.map((droit) => (
                  <span key={droit} className="text-[15px]">
                    {droit}
                  </span>
                ))
              ) : (
                <span className="text-[15px] text-ink-muted">Aucun droit d'administration.</span>
              )}
              <Link to="/refonte/admin/droits" className="mt-1 text-sm text-control no-underline">
                Gérer les droits
              </Link>
            </div>
          </Bloc>
        </aside>

        <div className="flex grow flex-col gap-4">
          <div className="grid grid-cols-3 gap-4">
            <Compteur titre="Commandes cette année" valeur={String(data.nbOrdersThisYear)} />
            <Compteur titre="Total cette année" valeur={euros(data.totalThisYear)} />
            <Compteur titre="Permanences tenues" valeur={String(data.nbVolunteering)} />
          </div>

          <section className="overflow-hidden rounded-card border-[1.5px] border-line bg-card">
            <h2 className="m-0 border-b border-line px-5 py-4 font-display text-lg italic">Ses commandes</h2>

            <div className="grid grid-cols-[150px_1fr_130px_120px] gap-3 border-b border-line bg-tint/60 px-5 py-3 text-xs uppercase tracking-wider text-ink-muted">
              <span>Distribution</span>
              <span>Contenu</span>
              <span>État</span>
              <span className="text-right">Montant</span>
            </div>

            {data.orders.map((commande) => (
              <div
                key={commande.multiDistribId}
                className="grid grid-cols-[150px_1fr_130px_120px] items-center gap-3 border-b border-line px-5 py-3 last:border-b-0"
              >
                <span className="text-[15px]">{commande.dateLabel}</span>
                <span className="truncate text-[15px] text-ink-muted">{commande.summary}</span>
                <span className="text-sm">{etat(commande.past, commande.delivered)}</span>
                <span className="text-right font-display text-base">{euros(commande.total)}</span>
              </div>
            ))}

            {data.orders.length === 0 && (
              <p className="m-0 px-5 py-6 text-ink-muted">Cet adhérent n'a jamais commandé.</p>
            )}
          </section>
        </div>
      </div>
    </div>
  )
}

/** « Non retirée » est l'information qu'on vient chercher : elle ne vaut que
 *  pour une distribution passée. */
function etat(passee: boolean, remise: boolean) {
  if (!passee) return <span className="rounded-[3px] bg-action-soft px-2 py-1 text-action-ink">à venir</span>
  if (remise) return <span className="text-control">retirée</span>
  return <span className="text-ink-faint">non retirée</span>
}

function Bloc({ titre, children }: { titre: string; children: React.ReactNode }) {
  return (
    <section className="overflow-hidden rounded-card border-[1.5px] border-line bg-card">
      <h2 className="m-0 border-b border-line px-4 py-3.5 font-display text-[17px] italic">{titre}</h2>
      {children}
    </section>
  )
}

function Champ({ libelle, valeur }: { libelle: string; valeur: string }) {
  return (
    <div className="flex flex-col gap-0.5 border-b border-line px-4 py-3 last:border-b-0">
      <span className="text-xs text-ink-muted">{libelle}</span>
      <span className="text-[15px]">{valeur}</span>
    </div>
  )
}

function Compteur({ titre, valeur }: { titre: string; valeur: string }) {
  return (
    <div className="rounded-card border-[1.5px] border-line bg-card p-4">
      <p className="m-0 text-[13px] text-ink-muted">{titre}</p>
      <p className="m-0 mt-1 font-display text-[32px] leading-none">{valeur}</p>
    </div>
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center px-8 text-center text-ink-muted">{children}</div>
}

function initiales(nom: string) {
  return nom
    .split(' ')
    .filter(Boolean)
    .slice(0, 2)
    .map((mot) => mot.charAt(0).toUpperCase())
    .join('')
}

function euros(montant: number) {
  return montant.toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}
