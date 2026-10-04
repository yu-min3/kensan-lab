import { useEffect, useState } from 'react'

// Everything on this page arrives at runtime, and that is the point.
//
//   /api/config  — what somebody typed into the scaffolder form. It travelled
//                  form -> deploy/values.yaml -> pull request -> Argo CD ->
//                  environment variable -> here. Nothing was rebuilt.
//   /api/whoami  — who the caller is. The gateway asked the identity provider
//                  before this pod saw the request and put the answer in
//                  headers, so the application knows without owning any auth.
type Config = {
  appName: string
  theme: 'day' | 'night'
  message: string
}

// /api/load is the one endpoint that exists for the walkthrough rather than for
// the service: it occupies a core on request so that a Grafana panel has
// something to draw. The backend refuses unless DEMO_LOAD_ENABLED is set, and
// reports that in `enabled`, so this page simply does not offer the button when
// it is not wanted.
type LoadState = {
  enabled: boolean
  running: boolean
  remaining: number
  seconds: number
}

type WhoAmI = {
  authenticated: boolean
  user: string | null
  email: string | null
  groups: string[]
  headers: Record<string, string>
}

export default function App() {
  const [config, setConfig] = useState<Config | null>(null)
  const [who, setWho] = useState<WhoAmI | null>(null)
  const [load, setLoad] = useState<LoadState | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    Promise.all([
      fetch('api/config').then((r) => r.json()),
      fetch('api/whoami').then((r) => r.json()),
    ])
      .then(([c, w]) => {
        setConfig(c)
        setWho(w)
      })
      .catch(() => setFailed(true))
  }, [])

  useEffect(() => {
    if (config) document.title = config.appName
  }, [config])

  const readLoad = () => fetch('api/load').then((r) => r.json()).then(setLoad)

  useEffect(() => {
    readLoad().catch(() => setLoad(null))
  }, [])

  // Only while something is burning: one poll a second is what turns the button
  // into a countdown, and there is nothing to count when it is idle.
  useEffect(() => {
    if (!load?.running) return
    const id = setInterval(() => {
      readLoad().catch(() => {})
    }, 1000)
    return () => clearInterval(id)
  }, [load?.running])

  if (failed) {
    return (
      <main className="shell theme-day">
        <div className="card">
          <h1>Cannot reach the backend</h1>
          <p className="muted">
            The page loaded, so the container is serving. <code>/api/config</code> did
            not answer — look at the pod logs.
          </p>
        </div>
      </main>
    )
  }

  if (!config || !who) {
    return <main className="shell theme-day" />
  }

  const night = config.theme === 'night'

  return (
    <main className={`shell theme-${config.theme}`}>
      <div className="sky" aria-hidden="true">
        <div className={night ? 'moon' : 'sun'} />
        {night && (
          <div className="stars">
            {Array.from({ length: 40 }).map((_, i) => (
              <span
                key={i}
                style={{
                  left: `${(i * 37) % 100}%`,
                  top: `${(i * 53) % 60}%`,
                  animationDelay: `${(i % 8) * 0.4}s`,
                }}
              />
            ))}
          </div>
        )}
      </div>

      <section className="stage">
        <p className="eyebrow">{config.appName}</p>
        <h1 className="greeting">{config.message}</h1>
        <p className="sub">
          {night ? 'This service is wearing its night face.' : 'This service is wearing its day face.'}{' '}
          Nobody rebuilt it to make that happen.
        </p>

        <div className="card">
          <h2>Who you are</h2>
          {who.authenticated ? (
            <>
              <p className="who">
                <strong>{who.user ?? who.email}</strong>
                {who.email && who.user !== who.email && <span className="muted"> · {who.email}</span>}
              </p>
              {who.groups.length > 0 && (
                <p className="chips">
                  {who.groups.map((g) => (
                    <span className="chip" key={g}>
                      {g}
                    </span>
                  ))}
                </p>
              )}
              <p className="muted small">
                This application contains no authentication code. The gateway checked with
                the identity provider first and handed the answer over in headers:
              </p>
              <dl className="headers">
                {Object.entries(who.headers).map(([k, v]) => (
                  <div key={k}>
                    <dt>{k}</dt>
                    <dd>{v}</dd>
                  </div>
                ))}
              </dl>
            </>
          ) : (
            <p className="muted">
              No identity headers arrived. Either the gateway gate is off for this
              hostname, or you reached the pod directly.
            </p>
          )}
        </div>

        {load?.enabled && (
          <div className="card">
            <h2>Make it do some work</h2>
            <p className="muted small">
              This occupies one core for {load.seconds} seconds. Prometheus scrapes every
              30 seconds, so the CPU panel in Grafana starts to climb after a scrape or
              two — no second terminal, and no namespace to type.
            </p>
            <p className="load-row">
              <button
                className="load"
                onClick={() =>
                  fetch('api/load', { method: load.running ? 'DELETE' : 'POST' })
                    .then((r) => r.json())
                    .then(setLoad)
                    .catch(() => {})
                }
              >
                {load.running
                  ? `Stop — ${load.remaining}s left`
                  : `Generate load for ${load.seconds}s`}
              </button>
            </p>
          </div>
        )}

        <p className="trail">
          scaffolder form → <code>deploy/values.yaml</code> → pull request → Argo CD →
          environment variable → this page
        </p>
      </section>
    </main>
  )
}
