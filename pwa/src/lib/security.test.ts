import { describe, expect, it } from 'vitest'
import {
  isSafeUrl,
  isValidHexColor,
  safeColor,
  safeUrl,
  sanitizeCSVCell,
  validateMediaFile,
  validatePassword,
} from './security'

describe('security utilities', () => {
  describe('isValidHexColor', () => {
    it('accepts valid 6-digit hex colors', () => {
      expect(isValidHexColor('#C8F14B')).toBe(true)
      expect(isValidHexColor('#0A3B28')).toBe(true)
      expect(isValidHexColor('#ffffff')).toBe(true)
    })

    it('rejects invalid colors', () => {
      expect(isValidHexColor('C8F14B')).toBe(false)
      expect(isValidHexColor('#fff')).toBe(false)
      expect(isValidHexColor('#GGGAAA')).toBe(false)
      expect(isValidHexColor('url(//evil)')).toBe(false)
      expect(isValidHexColor(undefined)).toBe(false)
    })
  })

  describe('safeColor', () => {
    it('returns valid colors unchanged', () => {
      expect(safeColor('#C8F14B')).toBe('#C8F14B')
    })

    it('returns transparent for invalid colors', () => {
      expect(safeColor('javascript:alert(1)')).toBe('transparent')
      expect(safeColor(undefined)).toBe('transparent')
    })
  })

  describe('isSafeUrl', () => {
    it('accepts safe external https urls', () => {
      expect(isSafeUrl('https://example.com')).toBe(true)
    })

    it('accepts relative paths', () => {
      expect(isSafeUrl('/api/media/123')).toBe(true)
    })

    it('rejects javascript and data:text/html', () => {
      expect(isSafeUrl('javascript:alert(1)')).toBe(false)
      expect(isSafeUrl('data:text/html,<script>alert(1)</script>')).toBe(false)
      expect(isSafeUrl('vbscript:msgbox(1)')).toBe(false)
    })

    it('accepts data:image and data:video urls', () => {
      expect(isSafeUrl('data:image/png;base64,abc')).toBe(true)
      expect(isSafeUrl('data:video/mp4;base64,abc')).toBe(true)
    })
  })

  describe('safeUrl', () => {
    it('returns safe urls unchanged', () => {
      expect(safeUrl('https://example.com')).toBe('https://example.com')
    })

    it('returns empty string for unsafe urls', () => {
      expect(safeUrl('javascript:alert(1)')).toBe('')
    })
  })

  describe('validateMediaFile', () => {
    it('accepts small allowed image', () => {
      const file = new File(['x'], 'photo.jpg', { type: 'image/jpeg' })
      Object.defineProperty(file, 'size', { value: 1024 })
      expect(validateMediaFile(file).ok).toBe(true)
    })

    it('rejects oversized files', () => {
      const file = new File(['x'], 'photo.jpg', { type: 'image/jpeg' })
      Object.defineProperty(file, 'size', { value: 60 * 1024 * 1024 })
      const result = validateMediaFile(file)
      expect(result.ok).toBe(false)
      expect(result.error).toContain('50 MB')
    })

    it('rejects disallowed mime types', () => {
      const file = new File(['x'], 'doc.exe', { type: 'application/x-msdownload' })
      Object.defineProperty(file, 'size', { value: 1024 })
      const result = validateMediaFile(file)
      expect(result.ok).toBe(false)
      expect(result.error).toContain('imagens ou vídeos')
    })
  })

  describe('validatePassword', () => {
    it('accepts strong password', () => {
      expect(validatePassword('Senha123')).toEqual({ ok: true })
    })

    it('rejects short password', () => {
      expect(validatePassword('Se1').ok).toBe(false)
    })

    it('rejects password without uppercase', () => {
      expect(validatePassword('senha123').ok).toBe(false)
    })

    it('rejects password without number', () => {
      expect(validatePassword('SenhaSemNumero').ok).toBe(false)
    })
  })

  describe('sanitizeCSVCell', () => {
    it('removes formula injection characters', () => {
      expect(sanitizeCSVCell('=CMD|\'\!A1')).toBe("CMD|'!A1")
      expect(sanitizeCSVCell('+1+1')).toBe('1+1')
      expect(sanitizeCSVCell('-10')).toBe('10')
      expect(sanitizeCSVCell('@SUM(A1)')).toBe('SUM(A1)')
    })

    it('replaces newlines with spaces', () => {
      expect(sanitizeCSVCell('line1\nline2')).toBe('line1 line2')
    })
  })
})
