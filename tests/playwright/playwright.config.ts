import { defineConfig } from '@playwright/test';

const lostPetPort = process.env.LOSTPET_PORT || '8080';
const foundPetPort = process.env.FOUNDPET_PORT || '8081';
const webFrontendPort = process.env.WEB_FRONTEND_PORT || '8082';

process.env.LOSTPET_PORT = lostPetPort;
process.env.FOUNDPET_PORT = foundPetPort;
process.env.WEB_FRONTEND_PORT = webFrontendPort;

if (!process.env.BASE_URL) {
  process.env.BASE_URL = `http://localhost:${webFrontendPort}`;
}
if (!process.env.WEB_FRONTEND_URL) {
  process.env.WEB_FRONTEND_URL = process.env.BASE_URL;
}
if (!process.env.LOSTPET_SERVICE_URL) {
  process.env.LOSTPET_SERVICE_URL = `http://localhost:${lostPetPort}`;
}
if (!process.env.FOUNDPET_SERVICE_URL) {
  process.env.FOUNDPET_SERVICE_URL = `http://localhost:${foundPetPort}`;
}

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
      command: `cd ../.. && PORT=${lostPetPort} go run ./cmd/lostpet-service`,
      url: `http://localhost:${lostPetPort}/healthz`,
      reuseExistingServer: !process.env.CI,
      timeout: 30000,
    },
    {
      command: `cd ../.. && PORT=${foundPetPort} go run ./cmd/foundpet-service`,
      url: `http://localhost:${foundPetPort}/healthz`,
      reuseExistingServer: !process.env.CI,
      timeout: 30000,
    },
    {
      command: `cd ../.. && PORT=${webFrontendPort} go run ./cmd/web-frontend`,
      url: `http://localhost:${webFrontendPort}/healthz`,
      reuseExistingServer: !process.env.CI,
      timeout: 30000,
    },
  ],
  use: {
    baseURL: process.env.BASE_URL || `http://localhost:${webFrontendPort}`,
    trace: 'on-first-retry',
  },
});
