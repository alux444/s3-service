import { type ReactElement, type ReactNode } from 'react'

interface PageFrameProps {
  readonly title: string
  readonly description: string
  readonly children: ReactNode
}

interface JsonPanelProps {
  readonly title: string
  readonly payload: unknown
}

export const PageFrame = ({ title, description, children }: PageFrameProps): ReactElement => {
  return (
    <section className="workbench-page">
      <header className="workbench-page-header">
        <h1>{title}</h1>
        <p>{description}</p>
      </header>
      <div className="workbench-page-content">{children}</div>
    </section>
  )
}

export const JsonPanel = ({ title, payload }: JsonPanelProps): ReactElement => {
  const serializedPayload: string = JSON.stringify(payload, null, 2) ?? 'undefined'

  return (
    <article className="workbench-card">
      <h2>{title}</h2>
      <pre className="workbench-json">{serializedPayload}</pre>
    </article>
  )
}
