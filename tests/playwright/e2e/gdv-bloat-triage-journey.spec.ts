import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Gastric Dilatation-Volvulus (GDV / Bloat), Splenic Torsion & Trocar Decompression Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, splenic torsion & hyperlactatemia evaluation, cranial IV & trocar decompression protocol completion, gastropexy clearance issuance, and status filter verification', async ({
    page,
  }) => {
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

    // Step 1: Cockpit Load & Seeded Operations
    await page.goto(`${WEB_FRONTEND_URL}/gdv-bloat-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#gdv-bloat-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Gastric Dilatation-Volvulus (GDV / Bloat), Splenic Torsion & Trocar Decompression Triage Cockpit'
    );

    const titanCard = page.locator('#gdv-operations-list .gdv-op-card', {
      hasText: 'Titan (',
    });
    const baronCard = page.locator('#gdv-operations-list .gdv-op-card', {
      hasText: 'Baron (',
    });
    const duchessCard = page.locator('#gdv-operations-list .gdv-op-card', {
      hasText: 'Duchess (',
    });

    await expect(titanCard).toBeVisible();
    await expect(baronCard).toBeVisible();
    await expect(duchessCard).toBeVisible();

    // Step 2: Evaluate custom splenic torsion & hemoabdomen operation ("Apollo")
    await page.locator('#eval-pet-name').fill('Apollo');
    await page.locator('#eval-species').fill('Dog - Doberman Pinscher');
    await page.locator('#eval-weight-kg').fill('39.5');
    await page
      .locator('#eval-presentation-type')
      .selectOption('SPLENIC_TORSION_HEMOABDOMEN');
    await page.locator('#eval-lactate-mmol').fill('7.4');
    await page.locator('#eval-heart-rate').fill('188');
    await page.locator('#eval-girth-cm').fill('98.0');
    await page.locator('#eval-retching').check();
    await page.locator('#eval-cranial-iv').check();
    await page.locator('#eval-vpcs').check();

    await page.locator('#btn-submit-gdv-eval').click();

    const apolloInitialCard = page
      .locator('#gdv-operations-list .gdv-op-card', {
        hasText: 'Apollo (',
      })
      .first();
    await expect(apolloInitialCard).toBeVisible();
    const apolloOpId = await apolloInitialCard.getAttribute('data-op-id');
    expect(apolloOpId).toBeTruthy();

    const apolloCard = page.locator(
      `#gdv-operations-list .gdv-op-card[data-op-id="${apolloOpId}"]`
    );
    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL'
    );
    await expect(apolloCard.locator('.gdv-venacava-alert-banner')).toContainText('caudal vena cava');

    // Step 3: Complete all 4 cranial IV, trocar decompression, double-bubble X-ray & gastropexy protocol steps on Apollo's card
    for (const stepId of [
      'STEP-BILATERAL-CEPHALIC-IV-BOLUS',
      'STEP-TROCAR-OROGASTRIC-DECOMPRESS',
      'STEP-RIGHT-LATERAL-DOUBLE-BUBBLE-XRAY',
      'STEP-LIDOCAINE-VPC-GASTROPEXY-PREP',
    ]) {
      const cb = apolloCard.locator(`.gdv-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'TROCAR_DECOMPRESSED_DUAL_IV_STAGED'
    );

    // Step 4: Issue Gastropexy & Sinus Rhythm Clearance Token
    await apolloCard.locator('.gdv-issue-clearance-btn').click();

    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'GASTROPEXY_SINUS_RHYTHM_CLEARED'
    );
    await expect(apolloCard.locator('.gdv-clearance-banner')).toContainText('GDV-SAFE-');

    // Step 5: Filter by GASTROPEXY_SINUS_RHYTHM_CLEARED and verify Apollo and Duchess remain visible
    await page.locator('#filter-status').selectOption('GASTROPEXY_SINUS_RHYTHM_CLEARED');
    await expect(
      page.locator('#gdv-operations-list .gdv-op-card', { hasText: 'Apollo (' })
    ).toBeVisible();
    await expect(
      page.locator('#gdv-operations-list .gdv-op-card', { hasText: 'Duchess (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
