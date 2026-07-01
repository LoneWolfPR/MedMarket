import { useEffect, useState } from 'react'
import './App.css'

function App() {
  // Hitting /api/health through Traefik proves both routes work end to end:
  // the browser loads the frontend at / and reaches the backend at /api.
  const [health, setHealth] = useState('checking…')

  useEffect(() => {
    fetch('/api/health')
      .then((res) => res.json())
      .then((data: { status: string }) => setHealth(data.status))
      .catch(() => setHealth('unreachable'))
  }, [])

  return (
    <main>
      <h1>MedMarket</h1>
      <p>
        Backend health: <strong>{health}</strong>
      </p>
    </main>
  )
}

export default App
