import { describe, expect, it } from 'vitest'
import type { ChargeStatus } from '../api/types'
import { feePercentToBasisPoints, formatBasisPoints, paidCents, payableCents } from './charges'

const charge = (status: ChargeStatus, fee = 60) => ({ status, amount_cents: 6000, pix_fee_cents: fee, pix_total_cents: 6000 + fee })

describe('valores da cobrança', () => {
  it('a pagar é o total no Pix, com a taxa repassada', () => {
    expect(payableCents(charge('pending'))).toBe(6060)
    expect(payableCents(charge('overdue', 0))).toBe(6000)
  })

  it('paga pelo Mercado Pago inclui a taxa; baixa manual registra só o rateio', () => {
    expect(paidCents(charge('paid'))).toBe(6060)
    expect(paidCents(charge('manual_paid'))).toBe(6000)
  })

})

describe('taxa nas configurações', () => {
  it('mostra pontos-base como porcentagem', () => {
    expect(formatBasisPoints('99')).toBe('0,99')
    expect(formatBasisPoints('150')).toBe('1,50')
    expect(formatBasisPoints('')).toBe('')
  })

  it('converte a porcentagem digitada em pontos-base', () => {
    expect(feePercentToBasisPoints('0,99')).toBe('99')
    expect(feePercentToBasisPoints('1.5')).toBe('150')
    expect(feePercentToBasisPoints(' 0 ')).toBe('0')
    // valor inválido segue para a API recusar com a mensagem dela
    expect(feePercentToBasisPoints('abc')).toBe('abc')
  })
})
