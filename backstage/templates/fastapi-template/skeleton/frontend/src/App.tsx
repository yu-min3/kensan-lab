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

        <p className="trail">
          scaffolder form → <code>deploy/values.yaml</code> → pull request → Argo CD →
          environment variable → this page
        </p>
      </section>
    </main>
  )
}
