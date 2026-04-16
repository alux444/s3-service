import { useContext } from 'react'
import type { AppDependencies } from './app-dependencies'
import { appDependenciesReactContext } from './app-dependencies-react-context'

export const useAppDependencies = (): AppDependencies => {
  const dependencies: AppDependencies | undefined = useContext(appDependenciesReactContext)
  if (dependencies === undefined) {
    throw new Error('AppDependenciesProvider is required in the component tree.')
  }

  return dependencies
}
