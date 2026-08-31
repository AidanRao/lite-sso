<template>
  <ProfileSettingsSection title-id="audit-title" :title="auditCopy.title" :description="auditCopy.description">
    <div class="audit-toolbar">
      <form class="audit-search" @submit.prevent="reload">
        <details class="filter-menu">
          <summary class="filter-toggle">
            <ListFilter :size="15" aria-hidden="true" />
            {{ auditCopy.filters }}
            <span v-if="filterCount" class="filter-count">{{ filterCount }}</span>
            <ChevronDown :size="13" aria-hidden="true" />
          </summary>
          <div class="filter-panel">
            <label>{{ auditCopy.result }}
              <select v-model="filters.outcome" @change="reload">
                <option v-for="option in outcomeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>
            <label>{{ auditCopy.period }}
              <select v-model="filters.period" @change="reload">
                <option v-for="option in periodOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>
            <template v-if="filters.period === 'custom'">
              <label>{{ auditCopy.from }}<input v-model="filters.startDate" type="date" :min="dateBounds.min" :max="dateBounds.max" @change="reload" /></label>
              <label>{{ auditCopy.to }}<input v-model="filters.endDate" type="date" :min="filters.startDate || dateBounds.min" :max="dateBounds.max" @change="reload" /></label>
            </template>
            <button class="text-button" type="button" @click="clearFilters">{{ auditCopy.clear }}</button>
          </div>
        </details>
        <div class="search-field">
          <Search :size="17" aria-hidden="true" />
          <input v-model="searchDraft" type="search" :placeholder="auditCopy.search" :aria-label="auditCopy.search" @keydown.enter="submitSearch" />
          <button class="search-submit" type="submit" :aria-label="auditCopy.searchSubmit"><ArrowRight :size="16" aria-hidden="true" /></button>
        </div>
      </form>
      <button class="button refresh-button" type="button" :disabled="loading" @click="reload">
        <RefreshCw :size="15" :class="{ spinning: loading }" aria-hidden="true" />{{ auditCopy.refresh }}
      </button>
    </div>

    <div class="events-panel" :aria-busy="loading">
      <div class="events-heading">
        <h3>{{ auditCopy.recent }}</h3>
        <span>{{ activePeriod }}</span>
      </div>
      <div v-if="loading" class="events-state" role="status"><LoaderCircle :size="20" class="spinning" aria-hidden="true" />{{ auditCopy.loading }}</div>
      <div v-else-if="errorMessage && !events.length" class="events-state error-state" role="alert">
        <CircleAlert :size="23" aria-hidden="true" /><p>{{ errorMessage }}</p>
        <button class="button" type="button" @click="reload">{{ auditCopy.retry }}</button>
      </div>
      <div v-else-if="!events.length" class="events-state">
        <ScrollText :size="28" aria-hidden="true" /><p>{{ hasFilters ? auditCopy.noMatches : auditCopy.empty }}</p>
      </div>
      <ol v-else class="events-list">
        <li v-for="event in events" :key="event.id" class="event-row">
          <div class="event-avatar" aria-hidden="true"><img v-if="user?.avatar_url" :src="user.avatar_url" alt="" /><span v-else>{{ avatarInitial }}</span></div>
          <div class="event-content">
            <div class="event-title">
              <span class="event-user">{{ displayName }}</span><span class="title-dash" aria-hidden="true">—</span>
              <button class="event-action" type="button" :aria-expanded="expanded.has(event.id)" :aria-controls="`audit-${event.id}`" @click="toggleDetails(event.id)">{{ event.action }}</button>
              <span class="outcome" :class="outcomeClass(event.outcome)">
                <Check v-if="event.outcome === 'success'" :size="12" aria-hidden="true" />
                <Ban v-else-if="event.outcome === 'denied'" :size="12" aria-hidden="true" />
                <CircleAlert v-else :size="12" aria-hidden="true" />{{ auditOutcomeLabel(event.outcome) }}
              </span>
            </div>
            <p class="event-summary">{{ auditSummary(event) }}</p>
            <div class="event-meta">
              <span v-if="event.client_id" class="event-application" :title="auditCopy.applicationCurrent">
                <ApplicationLogo :label="auditApplicationName(event)" :src="event.application?.logo_url || ''" size="inline" />
                <span>{{ auditApplicationName(event) }}</span>
              </span>
              <span>{{ event.ip || auditCopy.unknownIP }}</span>
              <span>{{ event.device_label || auditCopy.unknownDevice }}</span>
              <time :datetime="event.occurred_at" :title="auditFullTime(event.occurred_at)">{{ auditRelativeTime(event.occurred_at, clock) }}</time>
              <button class="more-button" type="button" :aria-label="expanded.has(event.id) ? auditCopy.hideDetails : auditCopy.details" :aria-expanded="expanded.has(event.id)" :aria-controls="`audit-${event.id}`" @click="toggleDetails(event.id)"><Ellipsis :size="17" aria-hidden="true" /></button>
            </div>
            <dl v-if="expanded.has(event.id)" :id="`audit-${event.id}`" class="event-details">
              <div v-for="detail in auditDetailRows(event)" :key="detail.label"><dt>{{ detail.label }}</dt><dd>{{ detail.value }}</dd></div>
            </dl>
          </div>
        </li>
      </ol>
    </div>
    <div v-if="events.length && !loading" class="list-footer">
      <p v-if="errorMessage" class="footer-error" role="alert">{{ errorMessage }}</p>
      <button v-if="nextCursor" class="button load-more" type="button" :disabled="loadingMore" @click="loadMore">
        <LoaderCircle v-if="loadingMore" :size="15" class="spinning" aria-hidden="true" />{{ loadingMore ? auditCopy.loadingMore : errorMessage ? auditCopy.retry : auditCopy.loadMore }}
      </button>
      <span v-else>{{ auditCopy.end }}</span>
    </div>
  </ProfileSettingsSection>
</template>

<script setup>
import { computed, inject, onBeforeUnmount, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Ban, Check, ChevronDown, CircleAlert, Ellipsis, ListFilter, LoaderCircle, RefreshCw, ScrollText, Search } from 'lucide-vue-next'
import ProfileSettingsSection from '../../components/profile/ProfileSettingsSection.vue'
import ApplicationLogo from '../../components/ApplicationLogo.vue'
import { userAPI } from '../../api/auth'
import { PROFILE_CONTEXT_KEY } from './profileContext'
import { auditApplicationName, auditCopy, auditDateBounds, auditDetailRows, auditFullTime, auditLoadError, auditOutcomeLabel, auditRelativeTime, auditSummary, buildAuditQuery, outcomeOptions, periodOptions } from '../../utils/auditLog'

const { user } = inject(PROFILE_CONTEXT_KEY)
const route = useRoute()
const router = useRouter()
const clock = ref(Date.now())
const dateBounds = computed(() => auditDateBounds(clock.value))
const filters = reactive({ q: '', outcome: '', period: '30', startDate: dateBounds.value.min, endDate: dateBounds.value.max })
const searchDraft = ref('')
const events = ref([])
const expanded = ref(new Set())
const nextCursor = ref('')
const errorMessage = ref('')
const loading = ref(false)
const loadingMore = ref(false)
const displayName = computed(() => user.value?.username || auditCopy.user)
const avatarInitial = computed(() => (user.value?.username || user.value?.email || auditCopy.user).slice(0, 1).toUpperCase())
const filterCount = computed(() => Number(Boolean(filters.outcome)) + Number(filters.period !== '30'))
const hasFilters = computed(() => Boolean(filters.q) || filterCount.value > 0)
const activePeriod = computed(() => periodOptions.find(option => option.value === filters.period)?.label)
let generation = 0
let controller
let appliedQuery
const ticker = setInterval(() => { clock.value = Date.now() }, 60000)

function outcomeClass(outcome) { return ['success', 'failure', 'denied'].includes(outcome) ? outcome : '' }
function toggleDetails(id) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

async function fetchPage(append) {
  if (append && (loading.value || loadingMore.value || !nextCursor.value)) return
  controller?.abort()
  controller = new AbortController()
  const current = ++generation
  errorMessage.value = ''
  if (append) loadingMore.value = true
  else {
    loading.value = true
    loadingMore.value = false
    events.value = []
    expanded.value = new Set()
    nextCursor.value = ''
  }
  try {
    if (!append) {
      clock.value = Date.now()
      filters.q = searchDraft.value.trim()
      try { appliedQuery = buildAuditQuery(filters, clock.value) }
      catch (error) { errorMessage.value = error.message; return }
    }
    const response = await userAPI.getAuditLogs({ ...appliedQuery, ...(append ? { cursor: nextCursor.value } : {}) }, controller.signal)
    if (current !== generation) return
    const data = response?.data || {}
    const items = Array.isArray(data.items) ? data.items : []
    const existing = new Set(append ? events.value.map(event => event.id) : [])
    events.value = [...(append ? events.value : []), ...items.filter(event => !existing.has(event.id))]
    nextCursor.value = data.next_cursor || ''
  } catch (error) {
    if (current !== generation) return
    if (error.status === 401) {
      router.replace({ path: '/login', query: { redirect: route.fullPath } })
      return
    }
    errorMessage.value = auditLoadError(error)
  } finally {
    if (current === generation) { loading.value = false; loadingMore.value = false }
  }
}
function reload() { return fetchPage(false) }
function loadMore() { return fetchPage(true) }
function submitSearch(event) {
  if (event.isComposing) return
  event.preventDefault()
  reload()
}
function clearFilters() {
  Object.assign(filters, { q: '', outcome: '', period: '30', startDate: dateBounds.value.min, endDate: dateBounds.value.max })
  searchDraft.value = ''
  reload()
}
onBeforeUnmount(() => { ++generation; controller?.abort(); clearInterval(ticker) })
reload()
</script>

<style scoped>
.audit-toolbar, .audit-search, .search-field, .filter-toggle, .button { display: flex; align-items: center; }
.audit-toolbar { gap: 12px; margin: 2px 0 4px; }
.audit-search { flex: 1; min-width: 0; border: 1px solid var(--profile-border); border-radius: 6px; background: var(--profile-surface); }
.filter-menu { position: relative; flex: none; }
.filter-toggle { min-height: 36px; gap: 6px; padding: 0 12px; border-right: 1px solid var(--profile-border); border-radius: 5px 0 0 5px; background: var(--profile-surface-subtle); cursor: pointer; list-style: none; font-size: 14px; font-weight: 600; }
.filter-toggle::-webkit-details-marker { display: none; }
.filter-count { min-width: 16px; border-radius: 10px; background: var(--profile-surface-hover); text-align: center; font-size: 12px; }
.search-field { flex: 1; min-width: 0; gap: 8px; padding: 0 10px; color: var(--profile-text-muted); }
.search-field input { width: 100%; min-width: 0; height: 36px; border: 0; outline: 0; background: transparent; color: var(--profile-text-strong); font: inherit; font-size: 14px; }
.search-field input::placeholder { color: var(--profile-text-muted); }
.audit-search:focus-within { border-color: var(--profile-accent); box-shadow: 0 0 0 2px var(--profile-focus-ring); }
.search-submit, .more-button { display: inline-flex; align-items: center; justify-content: center; flex: none; padding: 2px 4px; border: 0; border-radius: 4px; background: var(--profile-surface-subtle); color: var(--profile-text-muted); cursor: pointer; }
.search-submit { background: transparent; }
.filter-panel { position: absolute; top: calc(100% + 6px); left: 0; z-index: 5; display: grid; gap: 14px; width: 256px; box-sizing: border-box; padding: 16px; border: 1px solid var(--profile-border); border-radius: 8px; background: var(--profile-surface); box-shadow: var(--profile-shadow-soft); }
.filter-panel label { display: grid; gap: 7px; font-size: 13px; font-weight: 600; }
.filter-panel select, .filter-panel input { width: 100%; box-sizing: border-box; min-width: 0; height: 34px; padding: 4px 8px; border: 1px solid var(--profile-border); border-radius: 6px; background: var(--profile-surface); color: var(--profile-text-strong); font: inherit; font-size: 13px; }
.text-button { padding: 0; border: 0; background: transparent; color: var(--profile-accent); text-align: left; font: inherit; font-size: 13px; cursor: pointer; }
.button { justify-content: center; gap: 7px; min-height: 36px; padding: 5px 12px; border: 1px solid var(--profile-border-muted); border-radius: 6px; background: var(--profile-surface-subtle); color: var(--profile-text-strong); box-shadow: var(--profile-button-shadow); font: inherit; font-size: 14px; font-weight: 600; cursor: pointer; }
.button:hover:not(:disabled), .more-button:hover { background: var(--profile-surface-hover); }
.button:disabled { opacity: .6; cursor: default; }
.refresh-button { flex: none; }
button:focus-visible, summary:focus-visible, select:focus-visible, input:focus-visible { outline: 2px solid var(--profile-accent); outline-offset: 2px; }
.events-panel { overflow: hidden; border: 1px solid var(--profile-border); border-radius: 6px; }
.events-heading { display: flex; justify-content: space-between; align-items: center; gap: 12px; min-height: 48px; padding: 0 20px; background: var(--profile-surface-subtle); }
.events-heading h3 { margin: 0; font-size: 14px; font-weight: 600; }
.events-heading > span { color: var(--profile-text-muted); font-size: 12px; }
.events-list { margin: 0; padding: 0; list-style: none; }
.event-row { display: flex; gap: 14px; padding: 20px; border-top: 1px solid var(--profile-divider); }
.event-avatar { display: flex; align-items: center; justify-content: center; flex: 0 0 32px; width: 32px; height: 32px; overflow: hidden; border: 1px solid var(--profile-border-muted); border-radius: 50%; background: var(--profile-surface-subtle); color: var(--profile-text-muted); font-size: 13px; font-weight: 600; }
.event-avatar img { width: 100%; height: 100%; object-fit: cover; }
.event-content { flex: 1; min-width: 0; }
.event-title { display: flex; align-items: baseline; flex-wrap: wrap; gap: 4px 7px; line-height: 1.5; font-size: 14px; }
.event-user { font-weight: 600; overflow-wrap: anywhere; }
.title-dash { color: var(--profile-text-muted); }
.event-action { min-width: 0; padding: 0; border: 0; background: transparent; color: var(--profile-accent); font: inherit; font-weight: 600; text-align: left; overflow-wrap: anywhere; cursor: pointer; }
.event-action:hover { text-decoration: underline; }
.outcome { display: inline-flex; align-items: center; gap: 3px; padding: 0 6px; border: 1px solid var(--profile-border); border-radius: 20px; color: var(--profile-text-muted); font-size: 11px; line-height: 19px; white-space: nowrap; }
.outcome.success { color: var(--profile-success); border-color: var(--profile-success-border); }
.outcome.failure { color: var(--profile-danger); border-color: var(--profile-danger-border); }
.outcome.denied { color: var(--profile-warning); border-color: var(--profile-warning-border); }
.event-summary { margin: 4px 0; color: var(--profile-text-strong); font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
.event-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 5px 0; color: var(--profile-text-muted); font-size: 12px; line-height: 1.5; }
.event-meta > * + * { margin-left: 9px; padding-left: 9px; border-left: 1px solid var(--profile-divider); }
.event-meta > .more-button { padding: 0 4px; border: 0; }
.event-meta > span { overflow-wrap: anywhere; }
.event-application { display: inline-flex; align-items: center; gap: 5px; max-width: 100%; min-width: 0; }
.event-application > span:last-child { min-width: 0; overflow-wrap: anywhere; }
.event-details { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 24px; margin: 16px 0 0; padding: 16px; border: 1px solid var(--profile-border); border-radius: 6px; background: var(--profile-surface-subtle); }
.event-details > div { min-width: 0; }
.event-details dt { margin-bottom: 4px; color: var(--profile-text-muted); font-size: 12px; }
.event-details dd { margin: 0; color: var(--profile-text-strong); font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; white-space: pre-wrap; }
.events-state { display: flex; min-height: 240px; flex-direction: column; align-items: center; justify-content: center; gap: 12px; padding: 24px; border-top: 1px solid var(--profile-divider); color: var(--profile-text-muted); font-size: 14px; text-align: center; }
.events-state p { margin: 0; }
.error-state, .footer-error { color: var(--profile-danger); }
.list-footer { display: grid; justify-items: center; gap: 10px; color: var(--profile-text-muted); font-size: 12px; }
.footer-error { margin: 0; }
.load-more { min-width: 120px; }
.spinning { animation: audit-spin 1s linear infinite; }
@keyframes audit-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .spinning { animation: none; } }
@media (max-width: 620px) {
  .audit-toolbar { flex-wrap: wrap; gap: 8px; }
  .audit-search { flex-basis: 100%; }
  .refresh-button { margin-left: auto; }
  .filter-toggle { padding-inline: 8px; }
  .filter-toggle > svg:first-child { display: none; }
  .event-row { padding: 16px 12px; gap: 10px; }
  .event-avatar { width: 26px; height: 26px; flex-basis: 26px; }
  .events-heading { padding-inline: 12px; }
  .event-details { grid-template-columns: minmax(0, 1fr); padding: 12px; }
}
</style>
