import type { Clock } from './app-dependencies'

export const createSystemClock = (): Clock => {
  return {
    now: (): Date => new Date(),
  }
}
