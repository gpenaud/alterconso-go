/**
 * Écrire au groupe.
 *
 * Les destinataires viennent du serveur, qui les restreint aux droits de
 * l'utilisateur : un adhérent n'y trouve que son responsable de groupe et le
 * responsable technique. Cette liste n'est pas qu'un affichage — c'est elle qui
 * borne l'envoi côté serveur.
 */
import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { fetchRecipients, sendMessage } from '../../api/messages'

export function MessageRefonte() {
  const navigate = useNavigate()
  const [destinataire, setDestinataire] = useState<string>('')
  const [sujet, setSujet] = useState('')
  const [corps, setCorps] = useState('')

  const { data: destinataires, isLoading } = useQuery({
    queryKey: ['message-recipients'],
    queryFn: fetchRecipients,
  })

  const envoi = useMutation({
    mutationFn: () => sendMessage({ recipients: destinataire, subject: sujet, body: corps }),
  })

  if (isLoading) return <Message>Chargement…</Message>

  if (envoi.isSuccess) {
    return (
      <div className="flex flex-col items-center gap-5 px-8 py-20 text-center">
        <svg viewBox="0 0 76 76" className="size-[72px] fill-none" aria-hidden="true">
          <circle cx="38" cy="38" r="35" className="stroke-control" strokeWidth={2.5} />
          <path d="M23 39l10 11 20-24" className="stroke-control" strokeWidth={3.5} strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        <h1 className="m-0 font-display text-[28px]">Message envoyé</h1>
        <p className="m-0 text-ink-muted">La réponse arrivera sur votre adresse.</p>
        <button
          type="button"
          onClick={() => navigate('/refonte')}
          className="min-h-[52px] rounded-control bg-control px-8 text-[17px] font-semibold text-card"
        >
          Retour à l'accueil
        </button>
      </div>
    )
  }

  const complet = destinataire !== '' && sujet.trim() !== '' && corps.trim() !== ''

  return (
    <div className="flex flex-col text-ink">
      <header className="flex items-center gap-3 bg-surface px-5 py-4">
        <button type="button" onClick={() => navigate(-1)} aria-label="Revenir" className="bg-transparent p-0">
          <svg viewBox="0 0 24 24" className="size-[22px] fill-none stroke-ink" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
            <path d="M15 6l-6 6 6 6" />
          </svg>
        </button>
        <h1 className="m-0 font-display text-xl">Écrire au groupe</h1>
      </header>

      <div className="flex flex-col gap-5 px-5 py-5">
        <fieldset className="m-0 flex flex-col gap-2.5 border-0 p-0">
          <legend className="mb-1 p-0 text-xs uppercase tracking-[0.1em] text-ink-muted">À qui écrivez-vous ?</legend>
          {(destinataires ?? []).map((option) => (
            <label
              key={option.value}
              className={`flex cursor-pointer items-center gap-3.5 rounded-card border-[1.5px] bg-card p-4 ${
                destinataire === option.value ? 'border-control' : 'border-line'
              }`}
            >
              <input
                type="radio"
                name="destinataire"
                value={option.value}
                checked={destinataire === option.value}
                onChange={() => setDestinataire(option.value)}
                className="size-5 accent-control"
              />
              <span className="grow text-base">{option.name}</span>
              {option.count > 1 && <span className="text-sm text-ink-muted">{option.count}</span>}
            </label>
          ))}
          {destinataires?.length === 0 && (
            <p className="m-0 text-ink-muted">Aucun destinataire ne vous est accessible.</p>
          )}
        </fieldset>

        <label className="flex flex-col gap-2">
          <span className="text-xs uppercase tracking-[0.1em] text-ink-muted">Sujet</span>
          <input
            type="text"
            value={sujet}
            onChange={(e) => setSujet(e.target.value)}
            className="rounded-control border-[1.5px] border-line bg-card px-3.5 py-3 text-base text-ink"
          />
        </label>

        <label className="flex flex-col gap-2">
          <span className="text-xs uppercase tracking-[0.1em] text-ink-muted">Message</span>
          <textarea
            value={corps}
            onChange={(e) => setCorps(e.target.value)}
            rows={9}
            className="rounded-control border-[1.5px] border-line bg-card px-3.5 py-3 text-base leading-relaxed text-ink"
          />
        </label>

        {envoi.isError && (
          <p className="m-0 rounded-control bg-action-soft px-3.5 py-2.5 text-sm text-action-ink">
            Le message n'a pas pu être envoyé. Réessayez dans un instant.
          </p>
        )}

        <button
          type="button"
          disabled={!complet || envoi.isPending}
          onClick={() => envoi.mutate()}
          className="min-h-[54px] self-center rounded-control bg-action px-9 text-[17px] font-semibold text-card disabled:opacity-40"
        >
          {envoi.isPending ? 'Envoi…' : 'Envoyer'}
        </button>
        <p className="m-0 text-center text-sm text-ink-muted">La réponse arrivera sur votre adresse.</p>
      </div>
    </div>
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center bg-canvas px-8 text-center text-ink-muted">{children}</div>
}
