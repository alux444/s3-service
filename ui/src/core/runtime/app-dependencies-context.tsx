import { type ReactElement, type ReactNode } from 'react'
import type { AppDependencies } from './app-dependencies'
import { appDependenciesReactContext } from './app-dependencies-react-context'

interface AppDependenciesProviderProps {
  readonly dependencies: AppDependencies
  readonly children: ReactNode
}

export const AppDependenciesProvider = ({
  dependencies,
  children,
}: AppDependenciesProviderProps): ReactElement => {
  return (
    <appDependenciesReactContext.Provider value={dependencies}>
      {children}
    </appDependenciesReactContext.Provider>
  )
}
