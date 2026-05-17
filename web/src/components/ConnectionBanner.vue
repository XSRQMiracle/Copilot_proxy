<template>
  <transition name="banner-slide">
    <div v-if="state !== 'hidden'" class="conn-banner" :class="state">
      <span class="conn-banner__dot" />
      <span class="conn-banner__text">
        {{ state === 'disconnected' ? t('connectionBanner.disconnected') : t('connectionBanner.reconnected') }}
      </span>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { useI18n } from '../i18n'

const { t } = useI18n()

const props = defineProps<{
  connected: boolean
}>()

type BannerState = 'hidden' | 'disconnected' | 'reconnected'
const state = ref<BannerState>('hidden')
let hideTimer: number | undefined

watch(
  () => props.connected,
  (val, prev) => {
    if (hideTimer) {
      clearTimeout(hideTimer)
      hideTimer = undefined
    }
    if (val && prev === false) {
      state.value = 'reconnected'
      hideTimer = window.setTimeout(() => {
        state.value = 'hidden'
      }, 5000)
    } else if (!val) {
      state.value = 'disconnected'
      hideTimer = window.setTimeout(() => {
        state.value = 'hidden'
      }, 5000)
    }
  },
)

onUnmounted(() => {
  if (hideTimer) clearTimeout(hideTimer)
})
</script>

<style scoped>
.conn-banner {
  position: fixed;
  top: 120px;
  right: 12px;
  z-index: 9999;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 14px 28px;
  border-radius: 12px;
  font-size: 20px;
  font-weight: 600;
  white-space: nowrap;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  pointer-events: none;
}

.conn-banner.disconnected {
  background: #e74c3c;
  color: #fff;
}

.conn-banner.reconnected {
  background: #27ae60;
  color: #fff;
}

.conn-banner__dot {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.8;
}

.conn-banner__text {
  line-height: 1;
}

/* Slide-in from right */
.banner-slide-enter-active {
  transition: all 0.35s cubic-bezier(0.16, 1, 0.3, 1);
}

.banner-slide-leave-active {
  transition: all 0.25s ease-in;
}

.banner-slide-enter-from {
  transform: translateX(120%);
  opacity: 0;
}

.banner-slide-leave-to {
  transform: translateX(120%);
  opacity: 0;
}
</style>
