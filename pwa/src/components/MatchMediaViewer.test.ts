import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import MatchMediaViewer from './MatchMediaViewer.vue'
import type { Media } from '../api/types'

const media: Media[] = [
  { id: 'photo-1', match_id: 'match-1', uploaded_by: 'user-1', uploader_name: 'Ana', type: 'photo', url: '/api/media/files/match-1/photo.jpg', caption: 'Lance da partida', created_at: '' },
  { id: 'video-1', match_id: 'match-1', uploaded_by: 'user-1', uploader_name: 'Ana', type: 'video', url: '/api/media/files/match-1/video.mp4', caption: '', created_at: '' },
]

afterEach(() => { document.body.innerHTML = '' })

describe('visualizador de mídias', () => {
  it('amplia a foto, navega para o vídeo e oferece download dos dois', async () => {
    const wrapper = mount(MatchMediaViewer, { props: { media, initialIndex: 0 } })
    await flushPromises()
    expect(document.querySelector('[role="dialog"] img')?.getAttribute('src')).toBe(media[0].url)
    expect(document.querySelector('a[download]')?.getAttribute('href')).toBe(media[0].url)
    const zoomButton = document.querySelector('[aria-label="Aumentar foto"]') as HTMLButtonElement
    zoomButton.click()
    await flushPromises()
    expect(document.querySelector('[aria-label="Ajustar foto à tela"]')?.textContent).toContain('150%')
    const enlargedPhoto = document.querySelector('[role="dialog"] img') as HTMLImageElement
    enlargedPhoto.click()
    await flushPromises()
    expect(document.querySelector('[aria-label="Ajustar foto à tela"]')?.textContent).toContain('100%')

    const nextButton = document.querySelector('[aria-label="Próxima mídia"]') as HTMLButtonElement
    nextButton.click()
    await flushPromises()
    const video = document.querySelector('[role="dialog"] video')
    expect(video?.getAttribute('src')).toBe(media[1].url)
    expect(video?.hasAttribute('controls')).toBe(true)
    expect(document.querySelector('a[download]')?.getAttribute('href')).toBe(media[1].url)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
})
