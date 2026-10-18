import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Milestone 13.1: Automated Pet Custody Chain, Escrow & Cryptographic Reunion Handover Ledger Journey', () => {
  test('Complete 6-Step Pet Custody Chain, Dual-Party PIN Escrow & Cryptographic Reunion Handover User Journey', async ({ page, request }) => {
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

    const petId = 'pet-e2e-custody-apollo';
    const petName = 'Apollo';
    const microchipId = '985141003322119';
    let createdChainId = '';
    let validHandoverPin = '';
    let genesisEntryHash = '';

    // -------------------------------------------------------------------------
    // Step 1: Navigate to /custody-ledger & Verify Seeded Custody Chains
    // -------------------------------------------------------------------------
    await test.step('Step 1: Navigate to /custody-ledger & Verify Seeded Custody Chains', async () => {
      await page.goto(`${WEB_FRONTEND_URL}/custody-ledger`);
      await page.waitForLoadState('networkidle');

      await expect(page.locator('#custody-ledger-main')).toBeVisible();
      await expect(page.locator('h1')).toContainText(
        'Automated Pet Custody Chain, Escrow & Cryptographic Reunion Handover Ledger'
      );

      const card101 = page.locator('#custody-chains-list .custody-chain-card[data-chain-id="chain-custody-101"]');
      await expect(card101).toBeVisible();
      await expect(card101).toContainText('Cooper');
      await expect(card101).toHaveAttribute('data-custody-stage', 'OWNER_REUNIFICATION_ESCROW');
      await expect(card101.locator('.badge-integrity-intact')).toContainText('3 Hops');

      const card102 = page.locator('#custody-chains-list .custody-chain-card[data-chain-id="chain-custody-102"]');
      await expect(card102).toBeVisible();
      await expect(card102).toContainText('Milo');
      await expect(card102).toHaveAttribute('data-custody-stage', 'VET_ER_HOLD');
      await expect(card102.locator('.badge-integrity-intact')).toContainText('2 Hops');

      expect(cspErrors).toEqual([]);
    });

    // -------------------------------------------------------------------------
    // Step 2: Initiate New Pet Custody Chain with Genesis SHA-256 Hash
    // -------------------------------------------------------------------------
    await test.step('Step 2: Initiate New Pet Custody Chain with Genesis SHA-256 Hash', async () => {
      await page.locator('#custody-pet-id').fill(petId);
      await page.locator('#custody-pet-name').fill(petName);
      await page.locator('#custody-species').selectOption('Dog');
      await page.locator('#custody-microchip').fill(microchipId);
      await page.locator('#custody-initial-stage').selectOption('FINDER_INTAKE');
      await page.locator('#custody-from-custodian').fill('Green Lake East Trail');
      await page.locator('#custody-to-custodian').fill('Officer Maya Lin (SAR K9 Handler)');
      await page.locator('#custody-to-role').fill('COMMUNITY_FINDER');
      await page.locator('#custody-location').fill('Green Lake Community Boathouse');
      await page.locator('#custody-notes').fill('Friendly Golden Retriever wearing reflective harness; microchip scanned');

      await page.locator('#btn-create-custody-chain').click();

      const apolloCard = page.locator('#custody-chains-list .custody-chain-card', { hasText: petId }).first();
      await expect(apolloCard).toBeVisible();
      await expect(apolloCard).toContainText(petName);
      await expect(apolloCard).toHaveAttribute('data-custody-stage', 'FINDER_INTAKE');
      await expect(apolloCard.locator('.badge-custody-microchip')).toContainText(microchipId);

      const chainIdAttr = await apolloCard.getAttribute('data-chain-id');
      expect(chainIdAttr).toBeTruthy();
      createdChainId = chainIdAttr!;

      const pinAttr = await apolloCard.getAttribute('data-handover-pin');
      expect(pinAttr).toMatch(/^\d{6}$/);
      validHandoverPin = pinAttr!;

      const hop1 = page.locator('#custody-hops-timeline .custody-hop-node[data-hop-index="1"]');
      await expect(hop1).toBeVisible();
      await expect(hop1).toContainText('#1 FINDER_INTAKE');
      await expect(hop1).toContainText('0000000000000000000000000000000000000000000000000000000000000000');

      const hop1Text = await hop1.innerText();
      const entryHashMatch = hop1Text.match(/EntryHash:\s*([0-9a-f]{64})/i);
      expect(entryHashMatch).toBeTruthy();
      genesisEntryHash = entryHashMatch![1];
    });

    // -------------------------------------------------------------------------
    // Step 3: Append Intermediate Custody Transfer Hop (OWNER_REUNIFICATION_ESCROW)
    // -------------------------------------------------------------------------
    await test.step('Step 3: Append Intermediate Custody Transfer Hop (OWNER_REUNIFICATION_ESCROW)', async () => {
      await expect(page.locator('#active-chain-select')).toHaveValue(createdChainId);

      await page.locator('#hop-stage').selectOption('OWNER_REUNIFICATION_ESCROW');
      await page.locator('#hop-to-custodian').fill('Seattle Animal Shelter Reunion Escrow');
      await page.locator('#hop-to-role').fill('ESCROW_CUSTODIAN');
      await page.locator('#hop-location').fill('2061 15th Ave W, Seattle, WA');
      await page.locator('#hop-notes').fill('Owner Dr. Marcus Vance notified via ISO microchip match; awaiting PIN handover');

      await page.locator('#btn-append-custody-hop').click();

      const apolloCard = page.locator(`#custody-chains-list .custody-chain-card[data-chain-id="${createdChainId}"]`);
      await expect(apolloCard).toHaveAttribute('data-custody-stage', 'OWNER_REUNIFICATION_ESCROW');
      await expect(apolloCard.locator('.badge-integrity-intact')).toContainText('2 Hops');

      const hop2 = page.locator('#custody-hops-timeline .custody-hop-node[data-hop-index="2"]');
      await expect(hop2).toBeVisible();
      await expect(hop2).toContainText('#2 OWNER_REUNIFICATION_ESCROW');
      await expect(hop2).toContainText('Seattle Animal Shelter Reunion Escrow');
      await expect(hop2).toContainText(`PrevHash: ${genesisEntryHash}`);
    });

    // -------------------------------------------------------------------------
    // Step 4: Verify Dual-Party Handover PIN Gate (403 Forbidden on Wrong PIN)
    // -------------------------------------------------------------------------
    await test.step('Step 4: Verify Dual-Party Handover PIN Gate (403 Forbidden on Wrong PIN)', async () => {
      await page.locator('#reunion-pin-input').fill('000000');
      await page.locator('#reunion-owner-name').fill('Dr. Marcus Vance');
      await page.locator('#reunion-proof-method').selectOption('MICROCHIP_SCAN_MATCH');
      await page.locator('#reunion-location').fill('Seattle Animal Shelter Reunion Desk');
      await page.locator('#btn-complete-reunion').click();

      const errorAlert = page.locator('#pin-gate-error-alert');
      await expect(errorAlert).toBeVisible();
      await expect(errorAlert).toContainText('Handover PIN rejected');

      const apolloCard = page.locator(`#custody-chains-list .custody-chain-card[data-chain-id="${createdChainId}"]`);
      await expect(apolloCard).toHaveAttribute('data-custody-stage', 'OWNER_REUNIFICATION_ESCROW');
    });

    // -------------------------------------------------------------------------
    // Step 5: Complete Cryptographic Reunion Handover with Valid 6-Digit Escrow PIN
    // -------------------------------------------------------------------------
    await test.step('Step 5: Complete Cryptographic Reunion Handover with Valid 6-Digit Escrow PIN', async () => {
      await page.locator('#reunion-pin-input').fill(validHandoverPin);
      await page.locator('#reunion-owner-name').fill('Dr. Marcus Vance');
      await page.locator('#reunion-proof-method').selectOption('MICROCHIP_SCAN_MATCH');
      await page.locator('#reunion-location').fill('Seattle Animal Shelter Reunion Desk');
      const waiverCheckbox = page.locator('#reunion-waiver-checkbox');
      if (!(await waiverCheckbox.isChecked())) {
        await waiverCheckbox.check();
      }

      await page.locator('#btn-complete-reunion').click();

      await expect(page.locator('#pin-gate-error-alert')).toBeHidden();

      const apolloCard = page.locator(`#custody-chains-list .custody-chain-card[data-chain-id="${createdChainId}"]`);
      await expect(apolloCard).toHaveAttribute('data-custody-stage', 'REUNITED_RELEASED');
      await expect(apolloCard.locator('.badge-custody-reunited')).toContainText('REUNITED_RELEASED');
      await expect(apolloCard.locator('.custody-reunion-cert-banner')).toContainText('CERT-REUNION-');
      await expect(apolloCard.locator('.custody-reunion-cert-banner')).toContainText('Dr. Marcus Vance');

      const hop3 = page.locator('#custody-hops-timeline .custody-hop-node[data-hop-index="3"]');
      await expect(hop3).toBeVisible();
      await expect(hop3).toContainText('#3 REUNITED_RELEASED');
      await expect(hop3).toContainText('Dr. Marcus Vance');

      const certCard = page.locator('#reunion-certificate-card');
      await expect(certCard).toBeVisible();
      await expect(page.locator('#certificate-id-display')).toContainText('CERT-REUNION-');
      await expect(page.locator('#cert-owner')).toContainText('Dr. Marcus Vance');
      await expect(page.locator('#cert-proof')).toContainText('MICROCHIP_SCAN_MATCH');
    });

    // -------------------------------------------------------------------------
    // Step 6: Verify Cryptographic Audit Endpoint (/api/v1/custody/chains/{id}/verify)
    // -------------------------------------------------------------------------
    await test.step('Step 6: Verify Cryptographic Audit Endpoint (/api/v1/custody/chains/{id}/verify)', async () => {
      await page.locator('#btn-verify-chain-integrity').click();
      await expect(page.locator('#audit-result-badge')).toContainText('SHA-256 Verified (3 Hops)');

      const verifyRes = await request.get(
        `${WEB_FRONTEND_URL}/api/v1/custody/chains/${encodeURIComponent(createdChainId)}/verify`
      );
      expect(verifyRes.status()).toBe(200);
      const verifyBody = await verifyRes.json();
      expect(verifyBody.verifiedIntact).toBe(true);
      expect(verifyBody.hopCount).toBe(3);
      expect(verifyBody.latestEntryHashHex).toMatch(/^[0-9a-f]{64}$/);
      expect(cspErrors).toEqual([]);
    });
  });
});
