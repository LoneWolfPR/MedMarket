import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'

// ESLint 9+ "flat config": an array of config objects applied in order, each
// scoped by `files`. Replaces .eslintrc + `extends` string resolution.
export default tseslint.config(
  { ignores: ['dist'] },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      ...tseslint.configs.recommended,
      // v7 nests the flat-config variants under `.flat`; the top-level
      // `configs.recommended` is still the legacy eslintrc shape.
      reactHooks.configs.flat['recommended-latest'],
      // Warns when a module exports something other than a component, which
      // silently breaks Vite's fast refresh for that file.
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
  },
)
