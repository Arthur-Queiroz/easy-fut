import { describe, expect, it } from 'vitest'
import { formatPhoneBR, monthBounds } from './format'
import { cashTotals, centsToReaisInput, parseReais, reaisInputToCents, signedCashCents } from './guests'

describe('parseReais', () => {
  it.each([
    ['12,50', 1250],
    ['12.50', 1250],
    ['12', 1200],
    ['0,5', 50],
    ['R$ 1.234,56', 123456],
    ['1.234', 123400],
    [' 25 ', 2500],
  ])('%s → %d centavos', (input, cents) => {
    expect(parseReais(input)).toBe(cents)
  })

  it.each(['', 'abc', '12,345', '-5', '1,2,3'])('recusa %j', (input) => {
    expect(parseReais(input)).toBeNull()
  })
})

describe('campo de valor da configuração', () => {
  it('converte centavos para o campo e de volta', () => {
    expect(centsToReaisInput('2500')).toBe('25,00')
    expect(centsToReaisInput(undefined)).toBe('')
    expect(reaisInputToCents('25,25')).toBe('2525')
  })

  it('deixa texto inválido seguir para a API recusar', () => {
    expect(reaisInputToCents('vinte')).toBe('vinte')
  })
})

describe('caixa', () => {
  it('entrada soma; estorno e despesa subtraem', () => {
    expect(signedCashCents({ kind: 'guest_payment', amount_cents: 2500 })).toBe(2500)
    expect(signedCashCents({ kind: 'refund', amount_cents: 2500 })).toBe(-2500)
    expect(signedCashCents({ kind: 'expense', amount_cents: 1250 })).toBe(-1250)
    expect(
      cashTotals([
        { kind: 'guest_payment', amount_cents: 2500 },
        { kind: 'guest_payment', amount_cents: 2500 },
        { kind: 'expense', amount_cents: 1250 },
        { kind: 'refund', amount_cents: 2500 },
      ]),
    ).toEqual({ inCents: 5000, outCents: 3750 })
  })
})

describe('formatação', () => {
  it('formata celular E.164 e mantém outros formatos', () => {
    expect(formatPhoneBR('+5511987654321')).toBe('(11) 98765-4321')
    expect(formatPhoneBR('11 9999')).toBe('11 9999')
  })

  it('limites do mês', () => {
    expect(monthBounds('2026-02')).toEqual({ from: '2026-02-01', to: '2026-02-28' })
    expect(monthBounds('2026-10')).toEqual({ from: '2026-10-01', to: '2026-10-31' })
  })
})
