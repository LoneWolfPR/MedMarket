import { useEffect, useState } from 'react'

function useDebounce<T>(value: T, delay: number): T {
  const [input, setInput] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setInput(value), delay)
    return () => clearTimeout(id)
  }, [value, delay])
  return input
}

function SearchableList({ items }: { items: string[] }) {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)
  return (
    <div>
      <label htmlFor="search-query">Search Query</label>
      <input
        id="search-query"
        type="text"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />
      <ul>
        {items
          .filter((val) => val.toLowerCase().includes(debouncedQuery.toLowerCase()))
          .map((item) => (
            <li key={item}>{item}</li>
          ))}
      </ul>
    </div>
  )
}

export default SearchableList
