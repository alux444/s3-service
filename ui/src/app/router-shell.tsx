import { Link, Outlet } from '@tanstack/react-router'
import { type ReactElement } from 'react'
import './router-shell.css'

type ShellRoutePath =
  | '/'
  | '/auth'
  | '/bucket-connections'
  | '/access-policies'
  | '/objects'
  | '/images'
  | '/requests'

interface ShellNavigationLink {
  readonly path: ShellRoutePath
  readonly label: string
}

const shellNavigationLinks: ShellNavigationLink[] = [
  { path: '/', label: 'Setup' },
  { path: '/auth', label: 'Auth' },
  { path: '/bucket-connections', label: 'Bucket Connections' },
  { path: '/access-policies', label: 'Access Policies' },
  { path: '/objects', label: 'Objects' },
  { path: '/images', label: 'Images' },
  { path: '/requests', label: 'Requests' },
]

const renderNavigationLinks = (): ReactElement[] => {
  return shellNavigationLinks.map(({ path, label }: ShellNavigationLink): ReactElement => {
    return (
      <Link
        key={path}
        to={path}
        className="shell-nav-link"
        activeProps={{ className: 'shell-nav-link shell-nav-link-active' }}
      >
        {label}
      </Link>
    )
  })
}

export const AppShellLayout = (): ReactElement => {
  return (
    <div className="app-shell">
      <header className="shell-header">
        <h1>S3 Service API Workbench</h1>
        <p>Run setup, auth, policy, object, and image flows from one place.</p>
      </header>
      <nav className="shell-nav" aria-label="Workbench sections">
        {renderNavigationLinks()}
      </nav>
      <main className="shell-content">
        <Outlet />
      </main>
    </div>
  )
}
