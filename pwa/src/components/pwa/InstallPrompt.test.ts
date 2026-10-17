import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import InstallPrompt from './InstallPrompt.vue'

type InstallPromptEvent = Event & {
  prompt: ReturnType<typeof vi.fn>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

function installPromptEvent(): InstallPromptEvent {
  const event = new Event('beforeinstallprompt', { cancelable: true }) as InstallPromptEvent
  event.prompt = vi.fn().mockResolvedValue(undefined)
  event.userChoice = Promise.resolve({ outcome: 'accepted' })
  return event
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('InstallPrompt', () => {
  it('shows an install action and triggers the browser prompt', async () => {
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: false }))
    const wrapper = mount(InstallPrompt)
    const event = installPromptEvent()

    window.dispatchEvent(event)
    await nextTick()

    expect(event.defaultPrevented).toBe(true)
    expect(wrapper.text()).toContain('Instale o app')

    const buttons = wrapper.findAll('button')
    await buttons[1].trigger('click')

    expect(event.prompt).toHaveBeenCalledOnce()
    await nextTick()
    expect(wrapper.find('aside').exists()).toBe(false)
  })

  it('does not show the prompt when running as an installed app', async () => {
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))
    const wrapper = mount(InstallPrompt)

    window.dispatchEvent(installPromptEvent())
    await nextTick()

    expect(wrapper.find('aside').exists()).toBe(false)
  })
})
