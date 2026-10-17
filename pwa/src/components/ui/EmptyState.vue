<template>
  <div class="flex flex-col items-center gap-2 px-6 py-10 text-center">
    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4" class="text-ink3">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7l4.7 3.4-1.8 5.6H9.1L7.3 10.4z" />
    </svg>
    <p class="text-sm text-ink2"><slot /></p>
  </div>
</template>
