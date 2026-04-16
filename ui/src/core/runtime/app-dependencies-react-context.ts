import { createContext } from 'react'
import type { AppDependencies } from './app-dependencies'

export const appDependenciesReactContext = createContext<AppDependencies | undefined>(undefined)
