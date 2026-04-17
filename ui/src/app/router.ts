import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import type { AppDependencies } from '../core/runtime/app-dependencies'
import {
  AccessPoliciesRouteScreen,
  AppShellLayout,
  AuthRouteScreen,
  BucketConnectionsRouteScreen,
  ImagesRouteScreen,
  ObjectsRouteScreen,
  RequestsRouteScreen,
  SetupRouteScreen,
} from './router-shell'

export interface AppRouterContext {
  readonly dependencies: AppDependencies
}

const rootRoute = createRootRouteWithContext<AppRouterContext>()({
  component: AppShellLayout,
})

const setupRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/',
  component: SetupRouteScreen,
})

const authRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/auth',
  component: AuthRouteScreen,
})

const bucketConnectionsRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/bucket-connections',
  component: BucketConnectionsRouteScreen,
})

const accessPoliciesRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/access-policies',
  component: AccessPoliciesRouteScreen,
})

const objectsRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/objects',
  component: ObjectsRouteScreen,
})

const imagesRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/images',
  component: ImagesRouteScreen,
})

const requestsRoute = createRoute({
  getParentRoute: (): typeof rootRoute => rootRoute,
  path: '/requests',
  component: RequestsRouteScreen,
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
