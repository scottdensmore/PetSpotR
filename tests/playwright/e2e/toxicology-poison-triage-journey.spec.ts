import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity & Lipid Emulsion (ILE) Poison Control Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, feline lily nephrotoxicity evaluation, antidote & IV diuresis protocol completion, toxicology clearance issuance, and status filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/toxicology-poison-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#toxicology-poison-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity & Lipid Emulsion (ILE) Poison Control Cockpit'
    );

    const waffleCard = page.locator('#poison-operations-list .poison-op-card', {
      hasText: 'Waffle (',
    });
    const banditCard = page.locator('#poison-operations-list .poison-op-card', {
      hasText: 'Bandit (',
    });
    const tiggerCard = page.locator('#poison-operations-list .poison-op-card', {
      hasText: 'Tigger (',
    });

    await expect(waffleCard).toBeVisible();
    await expect(banditCard).toBeVisible();
    await expect(tiggerCard).toBeVisible();

    // Step 2: Evaluate custom feline Lilium spp. nephrotoxin operation ("Cleo")
    await page.locator('#eval-pet-name').fill('Cleo');
    await page.locator('#eval-species').fill('Cat - Domestic Shorthair');
    await page.locator('#eval-weight-kg').fill('4.3');
    await page
      .locator('#eval-agent-type')
      .selectOption('LILIUM_SPP_FELINE_NEPHROTOXIN');
    await page.locator('#eval-dose-mgkg').fill('85.0');
    await page.locator('#eval-pt-ratio').fill('1.0');
    await page.locator('#eval-glucose-mgdl').fill('108.0');
    await page.locator('#eval-ingestion-mins').fill('35');
    await page.locator('#eval-charcoal-avoided').check();
    await page.locator('#eval-k3-avoided').check();

    await page.locator('#btn-submit-poison-eval').click();

    const cleoInitialCard = page
      .locator('#poison-operations-list .poison-op-card', {
        hasText: 'Cleo (',
      })
      .first();
    await expect(cleoInitialCard).toBeVisible();
    const cleoOpId = await cleoInitialCard.getAttribute('data-op-id');
    expect(cleoOpId).toBeTruthy();

    const cleoCard = page.locator(
      `#poison-operations-list .poison-op-card[data-op-id="${cleoOpId}"]`
    );
    await expect(cleoCard).toHaveAttribute(
      'data-status',
      'COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL'
    );
    await expect(cleoCard.locator('.poison-pitfall-alert-banner')).toContainText('xylitol');

    // Step 3: Complete all 4 emesis gate, Vitamin K1 / ILE, dextrose/NAC/48h diuresis, and 48h PT recheck protocol steps on Cleo's card
    for (const stepId of [
      'STEP-APOMORPHINE-ROPIFINIROLE-EMESIS-GATE',
      'STEP-PHYTONADIONE-K1-OR-20PCT-ILE-LIPID',
      'STEP-DEXTROSE-NAC-SAME-48HR-IV-DIURESIS',
      'STEP-48HR-PT-RECHECK-RENAL-HEPATIC-PANEL',
    ]) {
      const cb = cleoCard.locator(`.poison-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(cleoCard).toHaveAttribute(
      'data-status',
      'VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED'
    );

    // Step 4: Issue Toxicology & Poison Clearance Token
    await cleoCard.locator('.poison-issue-clearance-btn').click();

    await expect(cleoCard).toHaveAttribute(
      'data-status',
      'DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED'
    );
    await expect(cleoCard.locator('.poison-clearance-banner')).toContainText('POISON-SAFE-');

    // Step 5: Filter by DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED and verify Cleo and Tigger remain visible
    await page.locator('#filter-status').selectOption('DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED');
    await expect(
      page.locator('#poison-operations-list .poison-op-card', { hasText: 'Cleo (' })
    ).toBeVisible();
    await expect(
      page.locator('#poison-operations-list .poison-op-card', { hasText: 'Tigger (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
