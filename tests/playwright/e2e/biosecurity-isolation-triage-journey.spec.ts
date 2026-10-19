import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Canine Parvovirus, Panleukopenia, Distemper & Biosecurity Isolation Cohort Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, canine distemper & neutropenic sepsis evaluation, negative-pressure Virkon/AHP protocol completion, PCR-negative clearance issuance, and status filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/biosecurity-isolation-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#biosecurity-isolation-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Canine Parvovirus, Panleukopenia, Distemper & Biosecurity Isolation Cohort Cockpit'
    );

    const baxterCard = page.locator('#biosec-operations-list .biosec-op-card', {
      hasText: 'Baxter (',
    });
    const mittensCard = page.locator('#biosec-operations-list .biosec-op-card', {
      hasText: 'Mittens (',
    });
    const otisCard = page.locator('#biosec-operations-list .biosec-op-card', {
      hasText: 'Otis (',
    });

    await expect(baxterCard).toBeVisible();
    await expect(mittensCard).toBeVisible();
    await expect(otisCard).toBeVisible();

    // Step 2: Evaluate custom Canine Distemper & Neutropenic Sepsis operation ("Freya")
    await page.locator('#eval-pet-name').fill('Freya');
    await page.locator('#eval-species').fill('Dog - Siberian Husky Juvenile');
    await page.locator('#eval-weight-kg').fill('11.5');
    await page
      .locator('#eval-pathogen-type')
      .selectOption('CANINE_DISTEMPER_CDV_NEURO');
    await page.locator('#eval-neutrophils').fill('890');
    await page.locator('#eval-pressure-pa').fill('-6.0');
    await page.locator('#eval-contact-mins').fill('10');
    await page.locator('#eval-hemorrhagic').uncheck();
    await page.locator('#eval-footbath').check();
    await page.locator('#eval-ppe-enforced').check();

    await page.locator('#btn-submit-biosec-eval').click();

    const freyaInitialCard = page
      .locator('#biosec-operations-list .biosec-op-card', {
        hasText: 'Freya (',
      })
      .first();
    await expect(freyaInitialCard).toBeVisible();
    const freyaOpId = await freyaInitialCard.getAttribute('data-op-id');
    expect(freyaOpId).toBeTruthy();

    const freyaCard = page.locator(
      `#biosec-operations-list .biosec-op-card[data-op-id="${freyaOpId}"]`
    );
    await expect(freyaCard).toHaveAttribute(
      'data-status',
      'NEUTROPENIC_SEPSIS_BREACH_CRITICAL'
    );
    await expect(freyaCard.locator('.biosec-quat-alert-banner')).toContainText(
      'quaternary ammonium'
    );

    // Step 3: Complete all 4 negative-pressure, Virkon/AHP, neutropenic sepsis & PCR shedding protocol steps on Freya's card
    for (const stepId of [
      'STEP-ANTEROOM-PPE-FOOTBATH-LOCK',
      'STEP-AHP-VIRKON-10MIN-DECON',
      'STEP-NASOJEJUNAL-MAROPITANT-SEPSIS',
      'STEP-PCR-SHEDDING-CLEARANCE-BATH',
    ]) {
      const cb = freyaCard.locator(`.biosec-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(freyaCard).toHaveAttribute(
      'data-status',
      'NEGATIVE_PRESSURE_ISO_COHORT_STAGED'
    );

    // Step 4: Issue PCR-Negative Biosecurity Clearance Token
    await freyaCard.locator('.biosec-issue-clearance-btn').click();

    await expect(freyaCard).toHaveAttribute(
      'data-status',
      'PCR_NEGATIVE_DECON_CLEARED'
    );
    await expect(freyaCard.locator('.biosec-clearance-banner')).toContainText('BIOSEC-SAFE-');

    // Step 5: Filter by PCR_NEGATIVE_DECON_CLEARED and verify Freya and Otis remain visible
    await page.locator('#filter-status').selectOption('PCR_NEGATIVE_DECON_CLEARED');
    await expect(
      page.locator('#biosec-operations-list .biosec-op-card', { hasText: 'Freya (' })
    ).toBeVisible();
    await expect(
      page.locator('#biosec-operations-list .biosec-op-card', { hasText: 'Otis (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
