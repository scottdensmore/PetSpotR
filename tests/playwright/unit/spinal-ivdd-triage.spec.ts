import { test, expect, type Page } from '@playwright/test';

/**
 * Milestone 22.3: Acute Spinal Cord Injury, Intervertebral Disc Extrusion (IVDD),
 * Deep Pain Nociception, Schiff-Sherrington Posture & Hemilaminectomy Triage Cockpit
 * Deterministic Playwright Unit Suite
 */
test.describe('Spinal IVDD & Hemilaminectomy Triage Cockpit Unit Tests (/spinal-ivdd-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  const seededCard = (page: Page, opId: string) =>
    page.locator(`#ivdd-operations-grid .ivdd-op-card[data-op-id="${opId}"]`);

  test('1. renders initial seeded operations, KPI cards, and Spinal Nociception alert banner', async ({ page }) => {
    await page.goto('/spinal-ivdd-triage');

    await expect(page.locator('#spinal-ivdd-cockpit')).toBeVisible();
    await expect(page.locator('#ivdd-alert-banner')).toBeVisible();
    await expect(page.locator('#ivdd-alert-banner')).toContainText('CRITICAL SPINAL CORD & IVDD NOCICEPTION GUARD');

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-acute-triage')).toBeVisible();
    await expect(page.locator('#kpi-staged-hemi')).toBeVisible();
    await expect(page.locator('#kpi-cleared-postop')).toBeVisible();

    await expect(seededCard(page, 'IVDD-OP-101')).toHaveAttribute('data-status', 'ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE');
    await expect(seededCard(page, 'IVDD-OP-102')).toHaveAttribute('data-status', 'CT_MYELOGRAM_HEMILAMINECTOMY_STAGED');
    await expect(seededCard(page, 'IVDD-OP-103')).toHaveAttribute('data-status', 'POSTOP_NOCICEPTION_INTACT_BLADDER_CLEARED');
    await expect(page.locator('#active-op-title')).toContainText('Barnaby');
    await expect(page.locator('#active-op-status-badge')).toHaveText('ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE');
  });

  test('2. filters operations by status via #ivdd-status-filter', async ({ page }) => {
    await page.goto('/spinal-ivdd-triage');

    await page.selectOption('#ivdd-status-filter', 'ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE');
    await expect(seededCard(page, 'IVDD-OP-101')).toContainText('Barnaby');
    await expect(
      page.locator('#ivdd-operations-grid .ivdd-op-card:not([data-status="ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE"])')
    ).toHaveCount(0);

    await page.selectOption('#ivdd-status-filter', '');
    for (const opId of ['IVDD-OP-101', 'IVDD-OP-102', 'IVDD-OP-103']) {
      await expect(seededCard(page, opId)).toBeVisible();
    }
  });

  test('3. stages hemilaminectomy decompression and updates status to CT_MYELOGRAM_HEMILAMINECTOMY_STAGED', async ({ page }) => {
    await page.goto('/spinal-ivdd-triage');

    await page.fill('#stage-window-input', 'Emergent Decompression < 12h; Target MAP >= 85 mmHg');
    await page.fill('#stage-imaging-input', 'High-Field 1.5T Thoracolumbar MRI + Helical CT');
    await page.fill('#stage-site-input', 'Left T13-L1 Hemilaminectomy + Prophylactic L1-L2 Fenestration');
    await page.fill('#stage-bladder-input', 'UMN Bladder: Prazosin 0.05 mg/kg PO q8h + Diazepam 0.2 mg/kg IV');
    await page.fill('#stage-analgesia-input', 'Fentanyl CRI 3 mcg/kg/h + Ketamine CRI 2 mcg/kg/min + Gabapentin');
    await page.fill('#stage-crate-rest-input', 'Strict 4-6 week confined crate rest, PROM physiotherapy');
    await page.click('#stage-decompression-btn');

    await expect(page.locator('#active-op-status-badge')).toHaveText('CT_MYELOGRAM_HEMILAMINECTOMY_STAGED');
  });

  test('4. records a serial neurological & nociception check', async ({ page }) => {
    await page.goto('/spinal-ivdd-triage');

    await page.selectOption('#neuro-frankel-input', 'GRADE_3_NON_AMBULATORY_PARAPARESIS');
    await page.fill('#neuro-deep-pain-input', 'Intact Bilaterally — Conscious Head Turn Restored');
    await page.fill('#neuro-trunci-input', 'L2 (Caudal Improvement — Zero Cranial Migration)');
    await page.fill('#neuro-bladder-input', 'Normotonic Detrusor — Voluntary Voiding Stream Observed');
    await page.fill('#neuro-recorded-by-input', 'Dr. Elena Rostova, DACVIM (Neurology)');
    await page.fill('#neuro-note-input', 'Conscious deep pain returning');
    await page.click('#record-neuro-check-btn');

    await expect(page.locator('#neuro-checks-list')).toContainText('Conscious deep pain returning');
    await expect(page.locator('#active-frankel-grade')).toHaveText('GRADE_3_NON_AMBULATORY_PARAPARESIS');
  });

  test('5. enforces WCAG AAA > 7.1:1 contrast in dark and light themes for badges and banners', async ({ page }) => {
    await page.goto('/spinal-ivdd-triage');

    const getContrast = async (selector: string) => {
      return page.locator(selector).first().evaluate((el) => {
        const style = window.getComputedStyle(el);
        const parseRgb = (c: string) => {
          const m = c.match(/\d+(\.\d+)?/g);
          return m ? m.slice(0, 3).map(Number) : [0, 0, 0];
        };
        const lum = ([r, g, b]: number[]) => {
          const a = [r, g, b].map((v) => {
            v /= 255;
            return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
          });
          return a[0] * 0.2126 + a[1] * 0.7152 + a[2] * 0.0722;
        };
        const fg = lum(parseRgb(style.color));
        const bg = lum(parseRgb(style.backgroundColor));
        const brightest = Math.max(fg, bg);
        const darkest = Math.min(fg, bg);
        return (brightest + 0.05) / (darkest + 0.05);
      });
    };

    // Dark mode check
    await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'));
    expect(await getContrast('#ivdd-alert-banner')).toBeGreaterThan(7.1);
    expect(await getContrast('#active-op-status-badge')).toBeGreaterThan(7.1);

    // Light mode check
    await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'light'));
    expect(await getContrast('#ivdd-alert-banner')).toBeGreaterThan(7.1);
    expect(await getContrast('#active-op-status-badge')).toBeGreaterThan(7.1);
  });
});
