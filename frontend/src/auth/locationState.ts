export function readRedirect(state: unknown): string | undefined {
  return typeof state === 'object' &&
    state !== null &&
    'from' in state &&
    typeof state.from === 'string' &&
    state.from.startsWith('/') &&
    !state.from.startsWith('//')
    ? state.from
    : undefined
}
