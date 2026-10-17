/**
 * Utilitários de segurança para sanitizar dados que vêm da API
 * antes de serem renderizados no DOM, usados em URLs ou exportados.
 */

const HEX_COLOR_REGEX = /^#[0-9A-Fa-f]{6}$/

/** Valida se a string é uma cor hexadecimal de 6 dígitos (#RRGGBB). */
export function isValidHexColor(color: string | undefined | null): color is string {
  return typeof color === 'string' && HEX_COLOR_REGEX.test(color)
}

/** Retorna uma cor segura para uso em CSS; fallback transparente. */
export function safeColor(color: string | undefined | null): string {
  return isValidHexColor(color) ? color : 'transparent'
}

const ALLOWED_URL_SCHEMES = ['https:', 'http:', 'blob:', 'data:']

/**
 * Verifica se uma URL externa é segura para usar em href/src.
 * Permite apenas esquemas conhecidos e rejeita javascript:, vbscript:, file: etc.
 */
export function isSafeUrl(url: string | undefined | null): url is string {
  if (!url) return false
  try {
    const parsed = new URL(url, window.location.href)
    if (!ALLOWED_URL_SCHEMES.includes(parsed.protocol)) return false
    // data: só é permitido para imagens/vídeos (evita data:text/html)
    if (parsed.protocol === 'data:' && !parsed.pathname.startsWith('image/') && !parsed.pathname.startsWith('video/')) {
      return false
    }
    return true
  } catch {
    // Caminhos relativos (ex.: /api/media/xyz) são aceitos.
    return url.startsWith('/') && !url.startsWith('//')
  }
}

/** Retorna a URL sanitizada ou uma string vazia se for rejeitada. */
export function safeUrl(url: string | undefined | null): string {
  return isSafeUrl(url) ? url : ''
}

const MEDIA_SIZE_LIMIT_MB = 50
const MEDIA_SIZE_LIMIT_BYTES = MEDIA_SIZE_LIMIT_MB * 1024 * 1024

const ALLOWED_MEDIA_TYPES: Record<string, string[]> = {
  image: ['image/jpeg', 'image/png', 'image/webp', 'image/gif', 'image/heic'],
  video: ['video/mp4', 'video/quicktime', 'video/webm'],
}

export interface MediaValidationResult {
  ok: boolean
  error?: string
}

/** Valida tamanho e tipo real de arquivo de mídia antes do upload. */
export function validateMediaFile(file: File): MediaValidationResult {
  if (file.size > MEDIA_SIZE_LIMIT_BYTES) {
    return { ok: false, error: `O arquivo deve ter no máximo ${MEDIA_SIZE_LIMIT_MB} MB.` }
  }

  const category = file.type.startsWith('image/') ? 'image' : file.type.startsWith('video/') ? 'video' : null
  if (!category) {
    return { ok: false, error: 'Envie apenas imagens ou vídeos.' }
  }

  if (!ALLOWED_MEDIA_TYPES[category].includes(file.type)) {
    return {
      ok: false,
      error: `Formato não suportado. Aceitos: ${ALLOWED_MEDIA_TYPES.image.concat(ALLOWED_MEDIA_TYPES.video).join(', ')}.`,
    }
  }

  return { ok: true }
}

const MIN_PASSWORD_LENGTH = 8

/** Valida senha com os requisitos mínimos do sistema. */
export function validatePassword(password: string): { ok: true } | { ok: false; error: string } {
  if (password.length < MIN_PASSWORD_LENGTH) {
    return { ok: false, error: `A senha deve ter pelo menos ${MIN_PASSWORD_LENGTH} caracteres.` }
  }
  if (!/[A-Z]/.test(password)) {
    return { ok: false, error: 'A senha deve conter pelo menos uma letra maiúscula.' }
  }
  if (!/[a-z]/.test(password)) {
    return { ok: false, error: 'A senha deve conter pelo menos uma letra minúscula.' }
  }
  if (!/\d/.test(password)) {
    return { ok: false, error: 'A senha deve conter pelo menos um número.' }
  }
  return { ok: true }
}

const CSV_FORMULA_CHARS = /^[=+\-@\t\r]/

/** Sanitiza uma célula de CSV para prevenir CSV/formula injection. */
export function sanitizeCSVCell(value: unknown): string {
  let cell = String(value ?? '')
  // Remove caracteres de controle perigosos no início da célula.
  while (CSV_FORMULA_CHARS.test(cell)) {
    cell = cell.slice(1)
  }
  // Substitui quebras de linha que poderiam injetar novas linhas no CSV.
  cell = cell.replace(/\r?\n/g, ' ')
  return cell
}
