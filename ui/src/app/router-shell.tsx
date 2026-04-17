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

interface RouteScreenProps {
  readonly title: string
  readonly detail: string
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

const RouteScreen = ({ title, detail }: RouteScreenProps): ReactElement => {
  return (
    <section className="route-screen">
      <h1>{title}</h1>
      <p>{detail}</p>
    </section>
  )
}

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
        <p>Route shell ready. Endpoint workflows are next.</p>
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

export const SetupRouteScreen = (): ReactElement => {
  return <RouteScreen title="Setup" detail="Configure base URL and token in the next task." />
}

export const AuthRouteScreen = (): ReactElement => {
  return <RouteScreen title="Auth" detail="Validate JWT claims and auth-check flow in the next task." />
}

export const BucketConnectionsRouteScreen = (): ReactElement => {
  return <RouteScreen title="Bucket Connections" detail="Create and list bucket connections in the next task." />
}

export const AccessPoliciesRouteScreen = (): ReactElement => {
  return <RouteScreen title="Access Policies" detail="Upsert scoped access policies in the next task." />
}

export const ObjectsRouteScreen = (): ReactElement => {
  return <RouteScreen title="Objects" detail="Upload, delete, and presign operations land in the next task." />
}

export const ImagesRouteScreen = (): ReactElement => {
  return <RouteScreen title="Images" detail="List and delete image-by-id flows land in the next task." />
}

export const RequestsRouteScreen = (): ReactElement => {
  return <RouteScreen title="Requests" detail="Request history and diagnostics land in the next task." />
}
