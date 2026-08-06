import { useQuery } from '@tanstack/react-query'

type HealthResponse = { status: string }
function Home() {
  const health = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      let resp: Response
      try {
        resp = await fetch('/api/health')
      } catch (e: unknown) {
        console.error(e)
        throw new Error("error fetching health", { cause: e })
      }
      if (resp.ok) {
        const healthStatus: HealthResponse = await resp.json()
        return healthStatus.status
      } else {
        throw new Error("error fetching health");
      }
    }
  })

  let statusText: string
  if (health.isPending) {
    statusText = 'checking...'
  } else if (health.isError) {
    statusText = health.error.message
  } else {
    statusText = health.data
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Welcome to MedMarket</h1>
      <p>
        Backend health: <strong>{statusText}</strong>
      </p>
    </div>
  )
}

export default Home
