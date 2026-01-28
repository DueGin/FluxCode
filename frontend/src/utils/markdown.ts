import MarkdownIt from 'markdown-it'

function isSafeLink(url: string): boolean {
  const trimmed = url.trim()
  if (!trimmed) return false
  if (trimmed.startsWith('#')) return true
  if (trimmed.startsWith('/')) return true

  try {
    const parsed = new URL(trimmed)
    return ['http:', 'https:', 'mailto:'].includes(parsed.protocol)
  } catch {
    return false
  }
}

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true
})

md.validateLink = isSafeLink

const defaultLinkOpenRenderer =
  md.renderer.rules.link_open ||
  ((tokens, idx, options, env, self) => {
    return self.renderToken(tokens, idx, options)
  })

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const token = tokens[idx]
  token.attrSet('target', '_blank')
  token.attrSet('rel', 'noopener noreferrer')
  return defaultLinkOpenRenderer(tokens, idx, options, env, self)
}

export function renderMarkdownToHtml(markdown: string): string {
  return md.render(markdown || '')
}

