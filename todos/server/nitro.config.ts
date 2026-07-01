import { defineNitroConfig } from 'nitropack/config'
import { config } from 'dotenv'

config()

export default defineNitroConfig({
  compatibilityDate: '2026-04-23',
  runtimeConfig: {
    nitro: {
      envPrefix: '',
    },
    DATABASE_API_BASE: process.env.DATABASE_API_BASE,
  },
})
