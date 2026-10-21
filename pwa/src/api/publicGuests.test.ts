import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./client', () => ({ api: {}, errorMessage: () => 'mensagem da API' }))
import { httpStatus, publicErrorMessage, readBookingToken, saveBookingToken } from './publicGuests'

beforeEach(() => localStorage.clear())
afterEach(() => vi.restoreAllMocks())

describe('token da inscrição no localStorage', () => {
  it('guarda vários tokens por inscrição', () => {
    saveBookingToken('b1', 't1')
    saveBookingToken('b2', 't2')
    expect(readBookingToken('b1')).toBe('t1')
    expect(readBookingToken('b2')).toBe('t2')
    expect(readBookingToken('b3')).toBeNull()
  })

  it('JSON corrompido não quebra a leitura', () => {
    localStorage.setItem('easyfut-guest-bookings', '{nope')
    expect(readBookingToken('b1')).toBeNull()
    saveBookingToken('b1', 't1')
    expect(readBookingToken('b1')).toBe('t1')
  })

  it('storage indisponível não lança', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceeded')
    })
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError')
    })
    expect(() => saveBookingToken('b1', 't1')).not.toThrow()
    expect(readBookingToken('b1')).toBeNull()
  })
})

describe('erros das telas públicas', () => {
  it('429 vira "muitas tentativas"; o resto usa a mensagem da API', () => {
    expect(publicErrorMessage({ response: { status: 429 } })).toContain('Muitas tentativas')
    expect(publicErrorMessage({ response: { status: 409 } })).toBe('mensagem da API')
    expect(httpStatus(new Error('offline'))).toBeUndefined()
  })
})
