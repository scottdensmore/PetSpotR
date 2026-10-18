import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

/**
 * Helper to parse rgb/rgba string into [r, g, b]
 */
function parseRgb(rgbStr: string): [number, number, number] {
  const match = rgbStr.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
  if (!match) return [0, 0, 0];
  return [parseInt(match[1], 10), parseInt(match[2], 10), parseInt(match[3], 10)];
}

/**
 * Relative luminance per WCAG 2.1
 */
function relativeLuminance([r, g, b]: [number, number, number]): number {
  const sRGB = [r / 255, g / 255, b / 255];
  const [R, G, B] = sRGB.map((c) =>
    c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
  );
  return 0.2126 * R + 0.7152 * G + 0.0722 * B;
}

/**
 * Contrast ratio per WCAG 2.1
 */
function contrastRatio(fgStr: string, bgStr: string): number {
  const l1 = relativeLuminance(parseRgb(fgStr));
  const l2 = relativeLuminance(parseRgb(bgStr));
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

test.describe('Pet Custody Chain & Cryptographic Handover Cockpit UI (/custody-ledger)', () => {
  test('1. /custody-ledger renders seeded custody chains (chain-custody-101, chain-custody-102), SHA-256 hash-chain timeline (#custody-hops-timeline), and handover forms with zero CSP errors', async ({ page }) => {
    const cspErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error' && /Content Security Policy/i.test(msg.text())) {
        cspErrors.push(msg.text());
      }
    });
    page.on('pageerror', (err) => {
      if (/Content Security Policy/i.test(err.message)) {
        cspErrors.push(err.message);
      }
    });

    await page.goto(`${WEB_FRONTEND_URL}/custody-ledger`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#custody-ledger-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Automated Pet Custody Chain, Escrow & Cryptographic Reunion Handover Ledger'
    );

    const card101 = page.locator('#custody-chains-list .custody-chain-card[data-chain-id="chain-custody-101"]');
    const card102 = page.locator('#custody-chains-list .custody-chain-card[data-chain-id="chain-custody-102"]');
    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-custody-stage', 'OWNER_REUNIFICATION_ESCROW');
    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-custody-stage', 'VET_ER_HOLD');

    await expect(page.locator('#custody-hops-timeline')).toBeVisible();
    await expect(page.locator('#custody-hops-timeline .custody-hop-node').first()).toBeVisible();
    await expect(page.locator('#form-create-custody-chain')).toBeVisible();
    await expect(page.locator('#form-append-custody-hop')).toBeVisible();
    await expect(page.locator('#form-complete-reunion')).toBeVisible();
    await expect(page.locator('#reunion-certificate-card')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Initiating a new custody chain via #form-create-custody-chain adds the chain card to #custody-chains-list with a 6-digit data-handover-pin and genesis hop (#1 FINDER_INTAKE)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/custody-ledger`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const petName = `Baxter #${uniqueSuffix}`;
    const petId = `pet-custody-baxter-${uniqueSuffix}`;

    await page.locator('#custody-pet-id').fill(petId);
    await page.locator('#custody-pet-name').fill(petName);
    await page.locator('#custody-species').selectOption('Dog');
    await page.locator('#custody-microchip').fill('985141007766554');
    await page.locator('#custody-initial-stage').selectOption('FINDER_INTAKE');
    await page.locator('#custody-from-custodian').fill('Magnuson Park Off-Leash Perimeter');
    await page.locator('#custody-to-custodian').fill('Taylor Reed (Community Finder)');
    await page.locator('#custody-to-role').fill('COMMUNITY_FINDER');
    await page.locator('#custody-location').fill('Magnuson Park North Kiosk');
    await page.locator('#custody-notes').fill('Healthy Beagle wearing green bandana');

    await page.locator('#btn-create-custody-chain').click();

    const newCard = page.locator('#custody-chains-list .custody-chain-card', { hasText: petName });
    await expect(newCard).toBeVisible();
    await expect(newCard).toHaveAttribute('data-custody-stage', 'FINDER_INTAKE');

    const pin = await newCard.getAttribute('data-handover-pin');
    expect(pin).toMatch(/^\d{6}$/);

    const hop1 = page.locator('#custody-hops-timeline .custody-hop-node[data-hop-index="1"]');
    await expect(hop1).toBeVisible();
    await expect(hop1).toContainText('#1 FINDER_INTAKE');
    await expect(hop1).toContainText('0000000000000000000000000000000000000000000000000000000000000000');
  });

  test('3. Appending a transfer hop via #form-append-custody-hop adds Hop #2 to #custody-hops-timeline linked via PrevHash and updates the chain stage', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/custody-ledger`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const petName = `Scout #${uniqueSuffix}`;
    const petId = `pet-custody-scout-${uniqueSuffix}`;

    await page.locator('#custody-pet-id').fill(petId);
    await page.locator('#custody-pet-name').fill(petName);
    await page.locator('#btn-create-custody-chain').click();

    const newCard = page.locator('#custody-chains-list .custody-chain-card', { hasText: petName });
    await expect(newCard).toBeVisible();

    const hop1 = page.locator('#custody-hops-timeline .custody-hop-node[data-hop-index="1"]');
    await expect(hop1).toBeVisible();
    const hop1Text = await hop1.innerText();
    const entryHashMatch = hop1Text.match(/EntryHash:\s*([0-9a-f]{64})/i);
    expect(entryHashMatch).toBeTruthy();
    const genesisEntryHash = entryHashMatch![1];

    await page.locator('#hop-stage').selectOption('OWNER_REUNIFICATION_ESCROW');
    await page.locator('#hop-to-custodian').fill('Seattle Animal Shelter Reunion Escrow');
    await page.locator('#hop-to-role').fill('ESCROW_CUSTODIAN');
    await page.locator('#hop-location').fill('2061 15th Ave W, Seattle, WA');
    await page.locator('#hop-notes').fill('Microchip scanned & owner contacted; placed in reunion escrow');

    await page.locator('#btn-append-custody-hop').click();

    await expect(newCard).toHaveAttribute('data-custody-stage', 'OWNER_REUNIFICATION_ESCROW');

    const hop2 = page.locator('#custody-hops-timeline .custody-hop-node[data-hop-index="2"]');
    await expect(hop2).toBeVisible();
    await expect(hop2).toContainText('#2 OWNER_REUNIFICATION_ESCROW');
    await expect(hop2).toContainText(`PrevHash: ${genesisEntryHash}`);
  });

  test('4. Dual-Party PIN Gate enforcement: submitting #form-complete-reunion with wrong PIN "000000" displays #pin-gate-error-alert (403 Forbidden), and submitting with valid 6-digit data-handover-pin transitions chain to REUNITED_RELEASED and issues CERT-REUNION-...', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/custody-ledger`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const petName = `Rosie #${uniqueSuffix}`;
    const petId = `pet-custody-rosie-${uniqueSuffix}`;

    await page.locator('#custody-pet-id').fill(petId);
    await page.locator('#custody-pet-name').fill(petName);
    await page.locator('#btn-create-custody-chain').click();

    const chainCard = page.locator('#custody-chains-list .custody-chain-card', { hasText: petName });
    await expect(chainCard).toBeVisible();
    const validPin = await chainCard.getAttribute('data-handover-pin');
    expect(validPin).toMatch(/^\d{6}$/);

    // Attempt reunion with wrong PIN "000000"
    await page.locator('#reunion-pin-input').fill('000000');
    await page.locator('#reunion-owner-name').fill('Jordan Taylor');
    await page.locator('#reunion-proof-method').selectOption('MICROCHIP_SCAN_MATCH');
    await page.locator('#reunion-location').fill('Seattle Animal Shelter Reunion Desk');
    await page.locator('#btn-complete-reunion').click();

    const errorAlert = page.locator('#pin-gate-error-alert');
    await expect(errorAlert).toBeVisible();
    await expect(errorAlert).toContainText('Handover PIN rejected');
    await expect(chainCard).toHaveAttribute('data-custody-stage', 'FINDER_INTAKE');

    // Submit with valid 6-digit PIN
    await page.locator('#reunion-pin-input').fill(validPin!);
    await page.locator('#btn-complete-reunion').click();

    await expect(errorAlert).toBeHidden();
    await expect(chainCard).toHaveAttribute('data-custody-stage', 'REUNITED_RELEASED');
    await expect(chainCard.locator('.custody-reunion-cert-banner')).toContainText('CERT-REUNION-');
    await expect(page.locator('#certificate-id-display')).toContainText('CERT-REUNION-');
  });

  test('5. WCAG AAA (> 7:1) contrast ratio check on .badge-custody-intake, .badge-custody-hold, .badge-custody-escrow, .badge-custody-reunited, and .badge-integrity-intact in both dark and light modes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/custody-ledger`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-custody-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-custody-intake" class="badge badge-custody-stage badge-custody-intake">FINDER_INTAKE</span>
        <span id="test-badge-custody-hold" class="badge badge-custody-stage badge-custody-hold">VET_ER_HOLD</span>
        <span id="test-badge-custody-escrow" class="badge badge-custody-stage badge-custody-escrow">OWNER_REUNIFICATION_ESCROW</span>
        <span id="test-badge-custody-reunited" class="badge badge-custody-stage badge-custody-reunited">REUNITED_RELEASED</span>
        <span id="test-badge-integrity-intact" class="badge badge-chain-integrity badge-integrity-intact">SHA-256 VERIFIED</span>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-custody-intake',
        '#test-badge-custody-hold',
        '#test-badge-custody-escrow',
        '#test-badge-custody-reunited',
        '#test-badge-integrity-intact',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7:1 contrast`).toBeGreaterThan(7.0);
      }
    }
  });
});
