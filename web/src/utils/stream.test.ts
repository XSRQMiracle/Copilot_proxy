import { describe, it, expect, vi } from 'vitest'
import { readStream } from './stream'

function mockReader(chunks: Uint8Array[], error?: Error): ReadableStreamDefaultReader<Uint8Array> {
  let idx = 0
  const reader: Partial<ReadableStreamDefaultReader<Uint8Array>> = {
    read: vi.fn().mockImplementation(() => {
      if (error) return Promise.reject(error)
      if (idx >= chunks.length) return Promise.resolve({ done: true as const, value: undefined as any })
      return Promise.resolve({ done: false as const, value: chunks[idx++] })
    }),
    cancel: vi.fn().mockResolvedValue(undefined),
    releaseLock: vi.fn(),
    closed: Promise.resolve(undefined),
  }
  return reader as ReadableStreamDefaultReader<Uint8Array>
}

function encode(text: string): Uint8Array {
  return new TextEncoder().encode(text)
}

describe('readStream', () => {
  it('parses SSE delta chunks', async () => {
    const deltas: string[] = []
    const reader = mockReader([
      encode('data: {"choices":[{"delta":{"content":"Hello"}}]}\n'),
      encode('data: {"choices":[{"delta":{"content":" World"}}]}\n'),
      encode('data: [DONE]\n'),
    ])

    await readStream(reader, {
      onDelta: (text) => deltas.push(text),
    })

    expect(deltas).toEqual(['Hello', ' World'])
  })

  it('handles empty content delta gracefully', async () => {
    const deltas: string[] = []
    const reader = mockReader([
      encode('data: {"choices":[{"delta":{"role":"assistant"}}]}\n'),
      encode('data: [DONE]\n'),
    ])

    await readStream(reader, {
      onDelta: (text) => deltas.push(text),
    })

    expect(deltas).toEqual([])
  })

  it('calls onDone on [DONE] signal', async () => {
    const onDone = vi.fn()
    const reader = mockReader([
      encode('data: {"choices":[{"delta":{"content":"ok"}}]}\n'),
      encode('data: [DONE]\n'),
    ])

    await readStream(reader, { onDelta: vi.fn(), onDone })
    expect(onDone).toHaveBeenCalledTimes(1)
  })

  it('calls onDone when reader is exhausted', async () => {
    const onDone = vi.fn()
    const reader = mockReader([
      encode('data: {"choices":[{"delta":{"content":"hello"}}]}\n'),
    ])

    await readStream(reader, { onDelta: vi.fn(), onDone })
    expect(onDone).toHaveBeenCalledTimes(1)
  })

  it('calls onError on malformed SSE', async () => {
    const onError = vi.fn()
    const reader = mockReader([
      encode('data: not-json\n'),
    ])

    await readStream(reader, { onDelta: vi.fn(), onError })
    expect(onError).toHaveBeenCalled()
    expect(onError.mock.calls[0][0].message).toContain('SSE parse error')
  })

  it('stops on abort signal', async () => {
    const onDelta = vi.fn()
    const controller = new AbortController()
    const reader = mockReader([
      encode('data: {"choices":[{"delta":{"content":"a"}}]}\n'),
      encode('data: {"choices":[{"delta":{"content":"b"}}]}\n'),
    ])

    controller.abort()
    await readStream(reader, { onDelta, signal: controller.signal })
    expect(onDelta).not.toHaveBeenCalled()
  })

  it('handles partial line buffering across chunks', async () => {
    const deltas: string[] = []
    const reader = mockReader([
      encode('data: {"choices":[{"delta":{"cont'),
      encode('ent":"Hello"}}]}\n'),
      encode('data: [DONE]\n'),
    ])

    await readStream(reader, {
      onDelta: (text) => deltas.push(text),
    })

    expect(deltas).toEqual(['Hello'])
  })

  it('calls onDone with usage if present', async () => {
    const onDone = vi.fn()
    const reader = mockReader([
      encode('data: {"choices":[{"delta":{"content":"hi"}}],"usage":{"total_tokens":10}}\n'),
    ])

    await readStream(reader, { onDelta: vi.fn(), onDone })
    expect(onDone).toHaveBeenCalledWith({ total_tokens: 10 })
  })

  it('does not call onDone twice when usage + DONE both present', async () => {
    const onDone = vi.fn()
    const reader = mockReader([
      encode('data: {"usage":{"total_tokens":10}}\n'),
      encode('data: [DONE]\n'),
    ])

    await readStream(reader, { onDelta: vi.fn(), onDone })
    expect(onDone).toHaveBeenCalledTimes(1)
    expect(onDone).toHaveBeenCalledWith({ total_tokens: 10 })
  })

  it('does not call onDone twice when DONE + reader both fire', async () => {
    const onDone = vi.fn()
    const reader = mockReader([
      encode('data: [DONE]\n'),
    ])

    await readStream(reader, { onDelta: vi.fn(), onDone })
    expect(onDone).toHaveBeenCalledTimes(1)
  })

  it('does not call onDone twice when multiple DONE signals are present', async () => {
    const onDone = vi.fn()
    const reader = mockReader([
      encode('data: [DONE]\n'),
      encode('data: [DONE]\n'),
    ])

    await readStream(reader, { onDelta: vi.fn(), onDone })
    expect(onDone).toHaveBeenCalledTimes(1)
  })

  it('usage-only chunk does not cause duplicate onDone', async () => {
    const onDelta = vi.fn()
    const onDone = vi.fn()
    const reader = mockReader([
      encode('data: {"choices":[{"delta":{"content":"hi"}}]}\n'),
      encode('data: {"usage":{"total_tokens":10}}\n'),
    ])

    await readStream(reader, { onDelta, onDone })
    expect(onDelta).toHaveBeenCalledWith('hi')
    expect(onDone).toHaveBeenCalledTimes(1)
    expect(onDone).toHaveBeenCalledWith({ total_tokens: 10 })
  })
})
