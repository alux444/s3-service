import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { App } from './App.tsx'
import { createAppRouter } from './app/router'
import { createBrowserDependencies } from './core/runtime/create-browser-dependencies'
import { AppDependenciesProvider } from './core/runtime/app-dependencies-context'

const getRootElement = (): HTMLElement => {
  const rootElement: HTMLElement | null = document.getElementById('root')
  if (rootElement === null) {
    throw new Error('Missing root element with id "root".')
  }

  return rootElement
}

const appDependencies = createBrowserDependencies()
const appRouter = createAppRouter({ dependencies: appDependencies })
const rootElement = getRootElement()

createRoot(rootElement).render(
  <StrictMode>
    <AppDependenciesProvider dependencies={appDependencies}>
      <App router={appRouter} />
    </AppDependenciesProvider>
  </StrictMode>,
)
