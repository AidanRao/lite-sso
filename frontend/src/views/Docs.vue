<template>
  <main class="docs-page">
    <header class="docs-header">
      <router-link class="brand" to="/login">
        <span class="brand-mark">L</span>
        <span>
          <strong>Lite SSO</strong>
          <small>项目文档</small>
        </span>
      </router-link>
      <router-link class="back-link" to="/login">返回登录</router-link>
    </header>

    <div class="docs-shell">
      <aside class="docs-sidebar">
        <p class="sidebar-label">文档目录</p>
        <nav v-if="documents.length" class="docs-nav">
          <router-link
            v-for="document in documents"
            :key="document.slug"
            :to="`/docs/${document.slug}`"
            :class="{ active: document.slug === activeDocument?.slug }"
          >
            {{ document.title }}
          </router-link>
        </nav>
        <p v-else class="empty-nav">暂无文档</p>
      </aside>

      <article v-if="activeDocument" class="markdown-body" v-html="renderedContent"></article>
      <section v-else class="docs-empty">
        <h1>未找到文档</h1>
        <p>请选择左侧目录中的文档。</p>
      </section>

      <aside v-if="outline.length" class="article-outline">
        <p class="sidebar-label">本文大纲</p>
        <nav class="outline-nav">
          <a
            v-for="heading in outline"
            :key="heading.id"
            :href="`#${heading.id}`"
            :class="[`level-${heading.level}`, { active: heading.id === activeHeadingID }]"
            @click.prevent="scrollToHeading(heading.id)"
          >
            {{ heading.title }}
          </a>
        </nav>
      </aside>
    </div>
  </main>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import MarkdownIt from 'markdown-it'
import { bundledLanguages, getSingletonHighlighter } from 'shiki'
import { documents, findDocument } from '../docs'

const route = useRoute()
const renderedContent = ref('')
const outline = ref([])
const activeHeadingID = ref('')
let scrollFrame = 0

const slugifyHeading = (title) => title
  .toLowerCase()
  .replace(/`([^`]+)`/g, '$1')
  .replace(/[^\p{Script=Han}a-z0-9]+/gu, '-')
  .replace(/^-+|-+$/g, '')

const createHeadingID = (title, sequence, usedHeadingIDs) => {
  const numberedHeading = title.match(/^(\d+(?:\.\d+)*)[.、]?\s*(.*)$/)
  const numberPath = numberedHeading?.[1]?.replace(/\./g, '-')
  const titleSlug = slugifyHeading(numberedHeading?.[2] || title)
  const baseID = numberPath
    ? ['chapter', numberPath, titleSlug].filter(Boolean).join('-')
    : ['topic', titleSlug || sequence].join('-')

  let headingID = baseID
  let duplicateIndex = 2
  while (usedHeadingIDs.has(headingID)) {
    headingID = `${baseID}-${duplicateIndex}`
    duplicateIndex += 1
  }
  usedHeadingIDs.add(headingID)

  return headingID
}

const getHeadingIDFromHash = (hash) => {
  const rawHeadingID = hash.replace(/^#/, '')
  if (!rawHeadingID) {
    return ''
  }

  let headingID = rawHeadingID
  try {
    headingID = decodeURIComponent(rawHeadingID)
  } catch {
    headingID = rawHeadingID
  }
  return outline.value.some((heading) => heading.id === headingID) ? headingID : ''
}

const syncActiveHeadingFromHash = (shouldScroll) => {
  const headingID = getHeadingIDFromHash(window.location.hash)
  if (!headingID) {
    return false
  }

  activeHeadingID.value = headingID
  if (shouldScroll) {
    document.getElementById(headingID)?.scrollIntoView({
      behavior: 'auto',
      block: 'start'
    })
  }

  if (window.location.hash !== `#${headingID}`) {
    window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}#${headingID}`)
  }
  return true
}

const createMarkdown = (highlight) => {
  const markdown = new MarkdownIt({
    html: false,
    linkify: true,
    highlight
  })
  const renderFence = markdown.renderer.rules.fence

  markdown.renderer.rules.fence = (tokens, index, options, env, renderer) => {
    const language = tokens[index].info.trim().split(/\s+/)[0] || 'text'
    const label = markdown.utils.escapeHtml(language)
    const code = renderFence(tokens, index, options, env, renderer)
    return `<div class="code-block"><span class="code-language">${label}</span>${code}</div>`
  }

  markdown.renderer.rules.heading_open = (tokens, index, options, env, renderer) => {
    const level = Number(tokens[index].tag.slice(1))
    if (level === 2 || level === 3) {
      const title = tokens[index + 1].content
      const heading = {
        id: createHeadingID(title, env.outline.length + 1, env.usedHeadingIDs),
        level,
        title
      }
      tokens[index].attrSet('id', heading.id)
      env.outline.push(heading)
    }
    return renderer.renderToken(tokens, index, options)
  }

  return markdown
}

const getLanguages = (content) => {
  const languages = new Set()
  const fences = content.matchAll(/^```(\S+)/gm)

  for (const fence of fences) {
    const language = fence[1]
    if (Object.hasOwn(bundledLanguages, language)) {
      languages.add(language)
    }
  }

  return [...languages]
}

const plainMarkdown = createMarkdown()

const activeDocument = computed(() => {
  if (!documents.length) {
    return null
  }
  if (!route.params.slug) {
    return documents[0]
  }
  return findDocument(String(route.params.slug))
})

const updateActiveHeadingByScroll = () => {
  if (!outline.value.length) {
    return
  }

  const anchorTop = 120
  let currentHeadingID = outline.value[0]?.id || ''
  for (const heading of outline.value) {
    const element = document.getElementById(heading.id)
    if (!element) {
      continue
    }
    if (element.getBoundingClientRect().top <= anchorTop) {
      currentHeadingID = heading.id
    } else {
      break
    }
  }
  activeHeadingID.value = currentHeadingID
}

const queueActiveHeadingUpdate = () => {
  if (scrollFrame) {
    return
  }
  scrollFrame = window.requestAnimationFrame(() => {
    scrollFrame = 0
    updateActiveHeadingByScroll()
  })
}

const observeHeadings = () => {
  window.removeEventListener('scroll', queueActiveHeadingUpdate)
  if (!outline.value.length) {
    return
  }

  window.addEventListener('scroll', queueActiveHeadingUpdate, { passive: true })
  updateActiveHeadingByScroll()
}

const renderDocument = async (markdown, content) => {
  const env = {
    outline: [],
    usedHeadingIDs: new Set()
  }
  renderedContent.value = markdown.render(content, env)
  outline.value = env.outline
  const hasSyncedHash = syncActiveHeadingFromHash(false)
  if (!hasSyncedHash && !outline.value.some((heading) => heading.id === activeHeadingID.value)) {
    activeHeadingID.value = outline.value[0]?.id || ''
  }
  await nextTick()
  observeHeadings()
  syncActiveHeadingFromHash(true)
}

const scrollToHeading = (headingID) => {
  activeHeadingID.value = headingID
  document.getElementById(headingID)?.scrollIntoView({
    behavior: 'smooth',
    block: 'start'
  })
  window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}#${headingID}`)
}

watch(() => route.hash, async () => {
  await nextTick()
  syncActiveHeadingFromHash(true)
})

watch(activeDocument, async (document) => {
  if (!document) {
    renderedContent.value = ''
    outline.value = []
    window.removeEventListener('scroll', queueActiveHeadingUpdate)
    return
  }

  await renderDocument(plainMarkdown, document.content)

  const languages = getLanguages(document.content)
  const highlighter = await getSingletonHighlighter({
    themes: ['github-dark'],
    langs: languages
  })
  if (activeDocument.value?.slug !== document.slug) {
    return
  }

  const highlightedMarkdown = createMarkdown((code, language) => {
    const lang = Object.hasOwn(bundledLanguages, language) ? language : 'text'
    return highlighter.codeToHtml(code, {
      lang,
      theme: 'github-dark'
    })
  })
  await renderDocument(highlightedMarkdown, document.content)
}, { immediate: true })

onBeforeUnmount(() => {
  window.removeEventListener('scroll', queueActiveHeadingUpdate)
  if (scrollFrame) {
    window.cancelAnimationFrame(scrollFrame)
  }
})
</script>

<style scoped>
.docs-page {
  min-height: 100vh;
  background: #f8fafc;
  color: #172033;
}

.docs-header {
  height: 68px;
  padding: 0 max(24px, calc((100vw - 1240px) / 2));
  border-bottom: 1px solid #e2e8f0;
  background: #ffffff;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #172033;
  text-decoration: none;
}

.brand-mark {
  width: 38px;
  height: 38px;
  border-radius: 11px;
  background: linear-gradient(135deg, #0891b2, #164e63);
  display: grid;
  place-items: center;
  color: #ffffff;
  font-size: 21px;
  font-weight: 700;
}

.brand strong,
.brand small {
  display: block;
}

.brand strong {
  font-size: 16px;
}

.brand small {
  color: #64748b;
  font-size: 12px;
  margin-top: 1px;
}

.back-link {
  border: 1px solid #cbd5e1;
  border-radius: 9px;
  padding: 9px 15px;
  color: #334155;
  font-size: 14px;
  font-weight: 500;
  text-decoration: none;
}

.back-link:hover {
  border-color: #0891b2;
  color: #0891b2;
}

.docs-shell {
  width: min(1440px, calc(100% - 48px));
  margin: 0 auto;
  display: grid;
  grid-template-columns: 224px minmax(0, 1fr) 208px;
  gap: 32px;
  padding: 36px 0 68px;
}

.docs-sidebar,
.article-outline {
  position: sticky;
  top: 28px;
  align-self: start;
}

.sidebar-label {
  margin: 0 0 12px;
  color: #64748b;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
}

.docs-nav {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.docs-nav a {
  border-radius: 9px;
  padding: 11px 12px;
  color: #475569;
  font-size: 14px;
  line-height: 1.45;
  text-decoration: none;
}

.docs-nav a:hover,
.docs-nav a.active {
  background: #ecfeff;
  color: #0e7490;
}

.docs-nav a.active {
  font-weight: 600;
}

.empty-nav {
  color: #94a3b8;
  font-size: 14px;
}

.outline-nav {
  border-left: 1px solid #e2e8f0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-left: 14px;
}

.outline-nav a {
  border-radius: 6px;
  color: #64748b;
  font-size: 13px;
  line-height: 1.4;
  padding: 6px 8px;
  text-decoration: none;
  transition: color 0.18s ease, background 0.18s ease;
}

.outline-nav a.level-3 {
  padding-left: 20px;
}

.outline-nav a:hover,
.outline-nav a.active {
  background: #ecfeff;
  color: #0e7490;
}

.outline-nav a.active {
  font-weight: 600;
}

.markdown-body,
.docs-empty {
  min-width: 0;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  background: #ffffff;
  padding: 44px 52px 60px;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.04);
}

.markdown-body :deep(h1) {
  margin: 0 0 28px;
  color: #0f172a;
  font-size: 32px;
  line-height: 1.25;
}

.markdown-body :deep(h2) {
  margin: 42px 0 16px;
  padding-bottom: 9px;
  border-bottom: 1px solid #e2e8f0;
  color: #0f172a;
  font-size: 23px;
  line-height: 1.35;
  scroll-margin-top: 28px;
}

.markdown-body :deep(h3) {
  margin: 29px 0 12px;
  color: #172033;
  font-size: 18px;
  line-height: 1.45;
  scroll-margin-top: 28px;
}

.markdown-body :deep(p),
.markdown-body :deep(li) {
  color: #334155;
  font-size: 15px;
  line-height: 1.78;
}

.markdown-body :deep(p) {
  margin: 12px 0;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 12px 0;
  padding-left: 26px;
}

.markdown-body :deep(ul) {
  list-style: disc outside;
}

.markdown-body :deep(ol) {
  list-style: decimal outside;
}

.markdown-body :deep(li) {
  padding-left: 3px;
}

.markdown-body :deep(a) {
  color: #0284c7;
  text-decoration: none;
}

.markdown-body :deep(a:hover) {
  text-decoration: underline;
}

.markdown-body :deep(blockquote) {
  margin: 18px 0;
  border-left: 4px solid #06b6d4;
  border-radius: 0 8px 8px 0;
  background: #ecfeff;
  padding: 10px 18px;
}

.markdown-body :deep(blockquote p) {
  margin: 3px 0;
}

.markdown-body :deep(table) {
  width: 100%;
  margin: 17px 0 24px;
  border-collapse: separate;
  border-spacing: 0;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  overflow: hidden;
  font-size: 14px;
}

.markdown-body :deep(th) {
  background: #f1f5f9;
  color: #172033;
  font-weight: 600;
  text-align: left;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  border-bottom: 1px solid #e2e8f0;
  padding: 10px 13px;
  vertical-align: top;
}

.markdown-body :deep(tr:last-child td) {
  border-bottom: 0;
}

.markdown-body :deep(code) {
  border-radius: 5px;
  background: #f1f5f9;
  padding: 2px 5px;
  color: #0e7490;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 0.92em;
}

.markdown-body :deep(.code-block) {
  position: relative;
  overflow: hidden;
  margin: 17px 0 24px;
  border: 1px solid #1e293b;
  border-radius: 11px;
  background: #0d1117;
}

.markdown-body :deep(.code-language) {
  position: absolute;
  top: 13px;
  right: 16px;
  color: #7d8590;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 11px;
  line-height: 1;
  text-transform: uppercase;
}

.markdown-body :deep(pre) {
  overflow-x: auto;
  margin: 0;
  background: #0f172a;
  padding: 20px 54px 20px 20px;
}

.markdown-body :deep(pre.shiki) {
  background: #0d1117 !important;
}

.markdown-body :deep(pre code) {
  background: transparent;
  padding: 0;
  color: #e2e8f0;
  font-size: 13px;
  line-height: 1.65;
}

.docs-empty h1 {
  margin-top: 0;
}

@media (max-width: 1140px) {
  .docs-shell {
    grid-template-columns: 224px minmax(0, 1fr);
  }

  .article-outline {
    display: none;
  }
}

@media (max-width: 860px) {
  .docs-shell {
    display: block;
    width: min(100% - 32px, 720px);
    padding-top: 24px;
  }

  .docs-sidebar,
  .article-outline {
    position: static;
    margin-bottom: 20px;
  }

  .markdown-body,
  .docs-empty {
    padding: 28px 22px 40px;
  }

  .markdown-body :deep(h1) {
    font-size: 27px;
  }
}
</style>
