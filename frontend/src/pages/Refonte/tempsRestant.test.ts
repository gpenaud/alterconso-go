import { describe, expect, it } from 'vitest'
import { tempsRestant } from './tempsRestant'

const maintenant = new Date('2026-08-25T10:00:00')

describe('tempsRestant', () => {
  it('compte en jours au-delà de vingt-quatre heures', () => {
    expect(tempsRestant('2026-08-27T20:00:00', maintenant)).toBe('2 jours')
    expect(tempsRestant('2026-08-26T12:00:00', maintenant)).toBe('1 jour')
  })

  it('bascule en heures le dernier jour', () => {
    expect(tempsRestant('2026-08-25T14:00:00', maintenant)).toBe('4 heures')
    expect(tempsRestant('2026-08-25T11:00:00', maintenant)).toBe('1 heure')
  })

  it('bascule en minutes la dernière heure', () => {
    expect(tempsRestant('2026-08-25T10:20:00', maintenant)).toBe('20 minutes')
  })

  // Un « 0 jour » afficherait une commande ouverte alors qu elle est close.
  it('ne rend rien quand la date est passée, absente ou illisible', () => {
    expect(tempsRestant('2026-08-25T09:59:00', maintenant)).toBeNull()
    expect(tempsRestant(undefined, maintenant)).toBeNull()
    expect(tempsRestant('pas une date', maintenant)).toBeNull()
  })
})
