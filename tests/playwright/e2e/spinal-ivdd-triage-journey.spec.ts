import { test, expect } from '@playwright/test';

/**
 * Milestone 22.3: End-to-End Journey Test for Spinal IVDD & Hemilaminectomy Triage Cockpit
 */
test.describe('Spinal IVDD & Hemilaminectomy Triage E2E Journey (/spinal-ivdd-triage)', () => {
  test('completes full triage lifecycle: alert -> admit -> stage hemilaminectomy -> record neuro check -> clear discharge', async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    await page.goto('/spinal-ivdd-triage');
    await expect(page.locator('#spinal-ivdd-cockpit')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Barnaby');

    // Connect SSE stream manually
    await page.click('#connect-sse-btn');
    await expect(page.locator('#sse-status-badge')).toHaveText(/SSE (Connected|Connecting)/);

    // Admit a new operation so the seeded operations stay untouched for specs
    // running in parallel against the same server.
    const petName = `Noodle ${Date.now()}`;
    await page.fill('#create-pet-name-input', petName);
    await page.click('#create-ivdd-btn');
    await expect(page.locator('#active-op-title')).toContainText(petName);

    // 1. Stage Hemilaminectomy Decompression
    await page.fill('#stage-window-input', 'Emergent Decompression < 12h (Grade 5 Deep Pain Absent); Target MAP >= 85 mmHg');
    await page.fill('#stage-imaging-input', 'High-Field 1.5T Thoracolumbar MRI + Helical CT');
    await page.fill('#stage-site-input', 'Left T13-L1 Hemilaminectomy + Prophylactic L1-L2 Fenestration');
    await page.fill('#stage-bladder-input', 'UMN Bladder: Prazosin 0.05 mg/kg PO q8h + Diazepam 0.2 mg/kg IV');
    await page.fill('#stage-analgesia-input', 'Fentanyl CRI 3 mcg/kg/h + Ketamine CRI 2 mcg/kg/min + Gabapentin');
    await page.fill('#stage-crate-rest-input', 'Strict 4-6 week confined crate rest, PROM physiotherapy');
    await page.click('#stage-decompression-btn');

    await expect(page.locator('#active-op-status-badge')).toHaveText('CT_MYELOGRAM_HEMILAMINECTOMY_STAGED');

    // 2. Record Serial Neuro & Nociception Check
    await page.selectOption('#neuro-frankel-input', 'GRADE_2_AMBULATORY_PARAPARESIS');
    await page.fill('#neuro-deep-pain-input', 'Intact Bilaterally — Conscious Head Turn & Vocalization Restored');
    await page.fill('#neuro-trunci-input', 'L2 (Caudal Improvement — Zero Cranial Migration)');
    await page.fill('#neuro-bladder-input', 'Normotonic Detrusor — Voluntary Voiding Stream Observed');
    await page.fill('#neuro-recorded-by-input', 'Dr. Elena Rostova, DACVIM (Neurology)');
    await page.fill('#neuro-note-input', 'Deep pain positive, voluntary tail wag and purposeful pelvic stepping');
    await page.click('#record-neuro-check-btn');

    await expect(page.locator('#neuro-checks-list')).toContainText('Deep pain positive, voluntary tail wag');

    // 3. Clear for Step-Down Crate Rest & Rehab
    await page.fill('#clear-discharged-by-input', 'Dr. Elena Rostova, DACVIM (Neurology)');
    await page.fill('#clear-summary-input', 'Deep pain positive, voluntary urination restored, cleared for 6-week crate rest and outpatient rehab.');
    await page.click('#clear-ivdd-btn');

    await expect(page.locator('#active-op-status-badge')).toHaveText('POSTOP_NOCICEPTION_INTACT_BLADDER_CLEARED');
    await expect(page.locator('#ivdd-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Deep pain positive, voluntary urination restored');

    expect(consoleErrors).toEqual([]);
  });
});
