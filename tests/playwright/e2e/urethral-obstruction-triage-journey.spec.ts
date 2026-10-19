import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, canine os penis calculus & hyperkalemic evaluation, calcium gluconate & retrohydropropulsion protocol completion, normokalemic clearance issuance, and status filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/urethral-obstruction-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#urethral-obstruction-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection Cockpit'
    );

    const sylvesterCard = page.locator('#urethral-operations-list .urethral-op-card', {
      hasText: 'Sylvester (',
    });
    const figaroCard = page.locator('#urethral-operations-list .urethral-op-card', {
      hasText: 'Figaro (',
    });
    const chesterCard = page.locator('#urethral-operations-list .urethral-op-card', {
      hasText: 'Chester (',
    });

    await expect(sylvesterCard).toBeVisible();
    await expect(figaroCard).toBeVisible();
    await expect(chesterCard).toBeVisible();

    // Step 2: Evaluate custom canine os penis calculus & hyperkalemic operation ("Buster")
    await page.locator('#eval-pet-name').fill('Buster');
    await page.locator('#eval-species').fill('Dog - Dalmatian Intact Male');
    await page.locator('#eval-weight-kg').fill('26.5');
    await page
      .locator('#eval-obstruction-cause')
      .selectOption('CANINE_OS_PENIS_CALCULUS_LODGMENT');
    await page.locator('#eval-potassium-mmol').fill('7.8');
    await page.locator('#eval-heart-rate').fill('124');
    await page.locator('#eval-bladder-cm').fill('12.4');
    await page.locator('#eval-absent-pwave').check();
    await page.locator('#eval-calcium-before-sedation').check();
    await page.locator('#eval-manual-expression-avoided').check();

    await page.locator('#btn-submit-urethral-eval').click();

    const busterInitialCard = page
      .locator('#urethral-operations-list .urethral-op-card', {
        hasText: 'Buster (',
      })
      .first();
    await expect(busterInitialCard).toBeVisible();
    const busterOpId = await busterInitialCard.getAttribute('data-op-id');
    expect(busterOpId).toBeTruthy();

    const busterCard = page.locator(
      `#urethral-operations-list .urethral-op-card[data-op-id="${busterOpId}"]`
    );
    await expect(busterCard).toHaveAttribute(
      'data-status',
      'HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL'
    );
    await expect(busterCard.locator('.urethral-cardioprotect-alert-banner')).toContainText('Calcium Gluconate');

    // Step 3: Complete all 4 calcium gluconate, sacrococcygeal block/retrohydropropulsion, closed diuresis & normokalemic clearance steps on Buster's card
    for (const stepId of [
      'STEP-CALCIUM-GLUCONATE-INSULIN-DEXTROSE',
      'STEP-SACROCOCCYGEAL-BLOCK-RETROHYDROPROPULSION',
      'STEP-CLOSED-COLLECTION-POST-OBSTRUCTIVE-DIURESIS',
      'STEP-PRAZOSIN-NORMO-K-CLEARANCE',
    ]) {
      const cb = busterCard.locator(`.urethral-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(busterCard).toHaveAttribute(
      'data-status',
      'CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED'
    );

    // Step 4: Issue Indwelling Closed Diuresis & Normokalemic Clearance Token
    await busterCard.locator('.urethral-issue-clearance-btn').click();

    await expect(busterCard).toHaveAttribute(
      'data-status',
      'INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED'
    );
    await expect(busterCard.locator('.urethral-clearance-banner')).toContainText('URETHRAL-SAFE-');

    // Step 5: Filter by INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED and verify Buster and Chester remain visible
    await page.locator('#filter-status').selectOption('INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED');
    await expect(
      page.locator('#urethral-operations-list .urethral-op-card', { hasText: 'Buster (' })
    ).toBeVisible();
    await expect(
      page.locator('#urethral-operations-list .urethral-op-card', { hasText: 'Chester (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
