import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  expect: {
    timeout: 5000,
  },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? 'dot' : 'list',
  webServer: [
    {
      command: 'cd ../.. && PORT=8080 go run ./cmd/lostpet-service',
      url: 'http://localhost:8080/healthz',
      reuseExistingServer: !process.env.CI,
      timeout: 30000,
    },
    {
      command: 'cd ../.. && PORT=8081 go run ./cmd/foundpet-service',
      url: 'http://localhost:8081/healthz',
      reuseExistingServer: !process.env.CI,
      timeout: 30000,
    },
    {
      command: 'cd ../.. && PORT=8082 go run ./cmd/web-frontend',
      url: 'http://localhost:8082/healthz',
      reuseExistingServer: !process.env.CI,
      timeout: 30000,
    },
  ],
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8082',
    trace: 'on-first-retry',
  },
});
