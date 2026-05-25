export type StreamChunk = { type: 'delta'; content: string } | { type: 'done'; usage?: Record<string, number> } | { type: 'error'; message: string }

export interface StreamOptions {
  onDelta: (text: string) => void
  onDone?: (usage?: Record<string, number>) => void
  onError?: (error: Error) => void
  signal?: AbortSignal
}

export async function readStream(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  options: StreamOptions,
): Promise<void> {
  const decoder = new TextDecoder()
  const { onDelta, onDone, onError, signal } = options
  let buffer = ''
  let finalized = false
  let latestUsage: Record<string, number> | undefined
  const complete = (usage?: Record<string, number>) => {
    if (finalized) return
    finalized = true
    onDone?.(usage ?? latestUsage)
  }

  try {
    while (true) {
      if (signal?.aborted) {
        await reader.cancel()
        return
      }

      const { done, value } = await reader.read()
      if (done) {
        complete()
        return
      }

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() ?? ''

      for (const line of lines) {
        const trimmed = line.trim()
        if (!trimmed.startsWith('data: ')) continue

        const data = trimmed.slice(6).trim()
        if (data === '[DONE]') {
          complete()
          return
        }

        try {
          const parsed = JSON.parse(data)
          const choices = Array.isArray(parsed.choices) ? parsed.choices : []
          const choice = choices[0]
          if (parsed.usage) {
            latestUsage = parsed.usage
          }
          if (choice?.delta?.content) {
            onDelta(choice.delta.content)
          }
          if (parsed.usage && choices.length === 0) {
            complete(parsed.usage)
            return
          }
          if (choice?.finish_reason) {
            complete()
            return
          }
        } catch {
          onError?.(new Error(`SSE parse error: ${data}`))
        }
      }
    }
  } catch (err) {
    if (signal?.aborted) return
    onError?.(err instanceof Error ? err : new Error(String(err)))
  } finally {
    reader.releaseLock()
  }
}

export async function createChatStream(
  model: string,
  messages: { role: string; content: string }[],
  signal?: AbortSignal,
): Promise<ReadableStreamDefaultReader<Uint8Array>> {
  const token = getAuthToken()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch('/api/chat/test', {
    method: 'POST',
    headers,
    body: JSON.stringify({ model, messages, stream: true }),
    signal,
  })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }

  if (!res.body) {
    throw new Error('Response body is null')
  }

  return res.body.getReader()
}

function getAuthToken(): string | null {
  try {
    return localStorage.getItem('admin_token')
  } catch {
    return null
  }
}
