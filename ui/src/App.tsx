import { RouterProvider } from '@tanstack/react-router'
import { type ReactElement } from 'react'
import type { AppRouter } from './app/router'

interface AppProps {
  readonly router: AppRouter
}

export const App = ({ router }: AppProps): ReactElement => {
  return <RouterProvider router={router} />
}
