import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Vitest runs without injected globals (see `globals` in vite.config.ts — it is
// left at its default of false), so Testing Library cannot find an `afterEach`
// to register its own cleanup against. Unmount rendered trees explicitly.
afterEach(cleanup)

// jsdom gives every test file one localStorage that persists across tests in
// that file. AuthProvider reads it during its lazy initializer, so a token left
// behind by one test would silently log the next one in.
afterEach(() => {
  localStorage.clear()
})
