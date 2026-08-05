import { useEffect, useState } from 'react'

function Home() {
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
    <div>
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Welcome to MedMarket</h1>
      <p>
        Backend health: <strong>{health}</strong>
      </p>
    </div>
  )
}

export default Home
