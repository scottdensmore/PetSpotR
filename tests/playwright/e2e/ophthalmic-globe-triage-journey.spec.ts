import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Ophthalmic Emergencies, Descemetoceles, Proptosed Globe, Acute Congestive Glaucoma (IOP > 40 mmHg) & Mannitol/Latanoprost Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Gizmo", staging Latanoprost/Mannitol IV/Tarsorrhaphy plan, logging rebound tonometry & Seidel check, issuing IOP normalized & globe protected discharge clearance, and filtering by IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/ophthalmic-globe-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#ophthalmic-globe-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Ophthalmic Emergencies, Descemetoceles, Proptosed Globe, Acute Congestive Glaucoma (IOP > 40 mmHg) & Mannitol/Latanoprost Cockpit'
    );

    await expect(
      page.locator('#ophthalmic-operations-list .ophthalmic-op-card', { hasText: 'Buster (' })
    ).toBeVisible();
    await expect(
      page.locator('#ophthalmic-operations-list .ophthalmic-op-card', { hasText: 'Pugsley (' })
    ).toBeVisible();
    await expect(
      page.locator('#ophthalmic-operations-list .ophthalmic-op-card', { hasText: 'Misty (' })
    ).toBeVisible();

    // Step 2: Create new operation "Gizmo"
    await page.locator('#create-incident-id').fill('INC-OG-9954');
    await page.locator('#create-pet-name').fill('Gizmo');
    await page.locator('#create-species').selectOption('CANINE_PRIMARY_CONGESTIVE_GLAUCOMA');
    await page.locator('#create-breed').fill('Shih Tzu');
    await page.locator('#create-weight-kg').fill('6.5');
    await page.locator('#create-iop-mmhg').fill('58');
    await page.locator('#create-fluorescein-pattern').selectOption('FLUORESCEIN_NEGATIVE_INTACT');
    await page.locator('#create-muscles-avulsed').fill('0');
    await page.locator('#create-anterior-luxation').uncheck();
    await page.locator('#create-consensual-plr').check();
    await page.locator('#btn-create-ophthalmic-op').click();

    const gizmoCard = page
      .locator('#ophthalmic-operations-list .ophthalmic-op-card', { hasText: 'Gizmo (' })
      .first();
    await expect(gizmoCard).toBeVisible();
    await expect(gizmoCard).toHaveAttribute('data-status', 'VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL');

    // Step 3: Stage Latanoprost & Mannitol IV plan for Gizmo
    await page.locator('#stage-weight-kg').fill('6.5');
    await page.locator('#stage-species').selectOption('CANINE_PRIMARY_CONGESTIVE_GLAUCOMA');
    await page.locator('#stage-iop-mmhg').fill('58');
    await page.locator('#stage-fluorescein-pattern').selectOption('FLUORESCEIN_NEGATIVE_INTACT');
    await page.locator('#stage-muscles-avulsed').fill('0');
    await page.locator('#stage-anterior-luxation').uncheck();
    await page.locator('#stage-consensual-plr').check();
    await page.locator('#btn-stage-plan').click();

    await expect(gizmoCard).toHaveAttribute('data-status', 'LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED');
    await expect(page.locator('#plan-latanoprost')).toContainText('ALLOWED (1 drop)');
    await expect(page.locator('#plan-dorzolamide')).toContainText('q8h');
    await expect(page.locator('#plan-mannitol')).toContainText('6.5 g IV');

    // Step 4: Log serial rebound tonometry, Seidel & dazzle reflex check for Gizmo
    await page.locator('#check-iop-mmhg').fill('17');
    await page.locator('#check-callsign').fill('DR-OPHTHO-28');
    await page.locator('#check-notes').fill('Post-Latanoprost & Mannitol 6.5 g IV tonometry: IOP reduced from 58 to 17 mmHg; Seidel negative.');
    await page.locator('#check-seidel-negative').check();
    await page.locator('#check-dazzle-present').check();
    await page.locator('#check-ecollar-verified').check();
    await page.locator('#btn-record-tonometry-check').click();

    await expect(page.locator('#tonometry-checks-list')).toContainText(
      'Post-Latanoprost & Mannitol 6.5 g IV tonometry: IOP reduced from 58 to 17 mmHg; Seidel negative.'
    );

    // Step 5: Issue IOP Normalized & Globe Protected Discharge Clearance for Gizmo
    await page.locator('#clearance-final-iop').fill('16');
    await page.locator('#clearance-vet-license').fill('VET-OPHTHO-88120');
    await page.locator('#clearance-summary').fill('IOP normalized at 16 mmHg; globe tectonic integrity confirmed; hard E-collar fitted and 5-minute drop spacing educated.');
    await page.locator('#clearance-tectonic-integrity').check();
    await page.locator('#clearance-ecollar-fitted').check();
    await page.locator('#clearance-drop-spacing-educated').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(gizmoCard).toHaveAttribute('data-status', 'IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED');
    await expect(page.locator('#ophthalmic-clearance-banner')).toBeVisible();
    await expect(page.locator('#ophthalmic-clearance-banner')).toContainText('VET-OPHTHO-88120');

    // Step 6: Filter by IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED and verify Gizmo and Misty remain visible
    await page.locator('#filter-status').selectOption('IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED');
    await expect(
      page.locator('#ophthalmic-operations-list .ophthalmic-op-card', { hasText: 'Gizmo (' })
    ).toBeVisible();
    await expect(
      page.locator('#ophthalmic-operations-list .ophthalmic-op-card', { hasText: 'Misty (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
