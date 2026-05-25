const markdownModules = import.meta.glob('../docs/*.md', {
  eager: true,
  import: 'default',
  query: '?raw'
})

const extractTitle = (content, fallback) => {
  const heading = content.match(/^#\s+(.+)$/m)
  return heading ? heading[1].trim() : fallback
}

export const documents = Object.entries(markdownModules)
  .map(([path, content]) => {
    const slug = path.split('/docs/').pop().replace(/\.md$/, '')
    return {
      slug,
      title: extractTitle(content, slug),
      content
    }
  })
  .sort((left, right) => left.title.localeCompare(right.title, 'zh-CN'))

export const findDocument = (slug) => documents.find((document) => document.slug === slug)
