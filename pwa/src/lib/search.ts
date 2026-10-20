/** Texto para busca: sem acento, minúsculo, espaços simples. */
export function normalizeSearch(s: string): string {
  return s
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim()
}

/** Opção de SearchSelect; a view pode acrescentar campos para o slot. */
export interface SearchOption {
  value: string
  label: string
  hint?: string
}
