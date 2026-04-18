import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import type { AppDependencies } from '../core/runtime/app-dependencies'
import { AccessPoliciesPage } from './pages/access-policies-page'
import { AuthPage } from './pages/auth-page'
import { BucketConnectionsPage } from './pages/bucket-connections-page'
import { ImagesPage } from './pages/images-page'
import { ObjectsPage } from './pages/objects-page'
import { RequestsPage } from './pages/requests-page'
import { SetupPage } from './pages/setup-page'
import { AppShellLayout } from './router-shell'

export interface AppRouterContext {
  readonly dependencies: AppDependencies
}

const rootRoute = createRootRouteWithContext<AppRouterContext>()({
  component: AppShellLayout,
})

const setupRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/',
  component: SetupPage,
})

const authRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/auth',
  component: AuthPage,
})

const bucketConnectionsRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/bucket-connections',
  component: BucketConnectionsPage,
})

const accessPoliciesRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/access-policies',
  component: AccessPoliciesPage,
})

const objectsRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/objects',
  component: ObjectsPage,
})

const imagesRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/images',
  component: ImagesPage,
})

const requestsRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/requests',
  component: RequestsPage,
})

const routeTree = rootRoute.addChildren([
  setupRoute,
  authRoute,
  bucketConnectionsRoute,
  accessPoliciesRoute,
  objectsRoute,
  imagesRoute,
  requestsRoute,
])

export const createAppRouter = (context: AppRouterContext): ReturnType<typeof createRouter> => {
  return createRouter({
    routeTree,
    context,
    defaultPreload: 'intent',
  })
}

export type AppRouter = ReturnType<typeof createAppRouter>

declare module '@tanstack/react-router' {
  interface Register {
    router: AppRouter
  }
}
