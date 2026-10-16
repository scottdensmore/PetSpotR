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

test.describe('Veterinary Clinic & Shelter HL7 FHIR R4 / PIMS Exchange Portal UI', () => {
  test('1. /veterinary/fhir renders clinics directory, handoff dispatch form, FHIR R4 inspector, and PIMS webhook simulator with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/veterinary/fhir`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#fhir-portal-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText('Veterinary Clinic & Shelter HL7 FHIR R4 / PIMS Exchange Portal');

    // Seeded 24/7 ER clinics in #clinics-list
    await expect(page.locator('#clinics-list')).toBeVisible();
    const clinicCards = page.locator('#clinics-list .clinic-partner-card');
    await expect(clinicCards.first()).toBeVisible();
    expect(await clinicCards.count()).toBeGreaterThanOrEqual(3);

    // Forms & panels
    await expect(page.locator('#form-register-clinic')).toBeVisible();
    await expect(page.locator('#form-dispatch-handoff')).toBeVisible();
    await expect(page.locator('#handoffs-list')).toBeVisible();
    await expect(page.locator('#fhir-inspector-panel')).toBeVisible();
    await expect(page.locator('#form-pims-webhook-sim')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Registering a custom clinic adds it to #clinics-list and #handoff-clinic-select', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/veterinary/fhir`);
    await page.waitForLoadState('networkidle');

    const clinicName = `Cascade Veterinary Emergency & Referral #${Date.now()}`;
    await page.locator('#clinic-name').fill(clinicName);
    await page.locator('#clinic-pims').selectOption('EZYVET');
    await page.locator('#clinic-phone').fill('(206) 555-0199');
    await page.locator('#clinic-address').fill('805 Madison St, Seattle, WA');
    await page.locator('#clinic-lat').fill('47.6092');
    await page.locator('#clinic-lng').fill('-122.3261');
    await page.locator('#btn-register-clinic').click();

    const newClinicCard = page.locator('#clinics-list .clinic-partner-card', { hasText: clinicName });
    await expect(newClinicCard).toBeVisible();
    await expect(newClinicCard.locator('.badge-pims-provider')).toContainText('EZYVET');

    const clinicId = await newClinicCard.getAttribute('data-clinic-id');
    expect(clinicId).toBeTruthy();

    const selectOption = page.locator(`#handoff-clinic-select option[value="${clinicId}"]`);
    await expect(selectOption).toContainText(clinicName);
  });

  test('3. Creating a triage assessment and dispatching a Pre-Arrival ER Handoff renders a .handoff-card with .badge-handoff-enroute and populates #fhir-bundle-json', async ({ page, request }) => {
    const testPetId = `pet-fhir-ui-${Date.now()}`;

    // Create a triage assessment via API first
    const triageRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/triage`, {
      data: {
        petId: testPetId,
        hubId: 'hub-seattle-center',
        medicId: 'medic-unit-01',
        medicName: 'Dr. Field Medic',
        species: 'Dog',
        weightKg: 24.5,
        vitals: {
          heartRateBpm: 195,
          respiratoryRateBpm: 52,
          temperatureF: 103.6,
          capillaryRefillSec: 3.0,
          mucousMembrane: 'MM_PALE',
          glasgowComaScale: 12,
        },
        trauma: {
          arterialHemorrhage: true,
        },
        traumaNotes: 'Acute arterial bleed and smoke inhalation',
      },
    });
    expect(triageRes.ok()).toBeTruthy();
    const assessment = await triageRes.json();

    await page.goto(`${WEB_FRONTEND_URL}/veterinary/fhir?petId=${encodeURIComponent(testPetId)}&assessmentId=${encodeURIComponent(assessment.assessmentId)}`);
    await page.waitForLoadState('networkidle');

    // Verify query param pre-population
    await expect(page.locator('#handoff-pet-id')).toHaveValue(testPetId);
    await expect(page.locator('#handoff-assessment-id')).toHaveValue(assessment.assessmentId);

    await page.locator('#handoff-clinic-select').selectOption('clinic-er-seattle-1');
    await page.locator('#handoff-transport-unit').fill('SAR MedEvac Unit #1');
    await page.locator('#handoff-eta-minutes').fill('12');
    await page.locator('#btn-dispatch-handoff').click();

    const handoffCard = page.locator('#handoffs-list .handoff-card').first();
    await expect(handoffCard).toBeVisible();
    await expect(handoffCard).toHaveAttribute('data-handoff-status', 'EN_ROUTE_PRE_ARRIVAL');
    await expect(handoffCard.locator('.badge-handoff-enroute')).toBeVisible();

    // Verify FHIR Bundle Inspector populated
    const fhirJsonViewer = page.locator('#fhir-bundle-json');
    await expect(fhirJsonViewer).toContainText('"resourceType": "Bundle"');
    await expect(fhirJsonViewer).toContainText('http://hl7.org/fhir/StructureDefinition/patient-animal');
    await expect(page.locator('#fhir-digest-badge')).not.toBeEmpty();

    // Simulate inbound PIMS webhook update to CLEARED_FOR_REUNIFICATION
    await page.locator('#pims-sim-status').selectOption('CLEARED_FOR_REUNIFICATION');
    await page.locator('#pims-sim-microchip').fill('985141002394811');
    await page.locator('#btn-send-pims-webhook').click();

    await expect(handoffCard).toHaveAttribute('data-handoff-status', 'CLEARED_FOR_REUNIFICATION');
    await expect(handoffCard.locator('.badge-handoff-cleared')).toBeVisible();
    await expect(handoffCard.locator('.reunion-clearance-banner')).toContainText('CLEARED FOR OWNER REUNIFICATION — Verified Microchip: 985141002394811');
  });

  test('4. WCAG AAA contrast ratios (> 7:1) on .badge-handoff-enroute, .badge-handoff-treatment, .badge-handoff-stabilized, and .badge-handoff-cleared in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/veterinary/fhir`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-handoff-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-enroute" class="badge-handoff-status badge-handoff-enroute">EN_ROUTE_PRE_ARRIVAL</span>
        <span id="test-badge-treatment" class="badge-handoff-status badge-handoff-treatment">IN_CLINIC_TREATMENT</span>
        <span id="test-badge-stabilized" class="badge-handoff-status badge-handoff-stabilized">STABILIZED</span>
        <span id="test-badge-cleared" class="badge-handoff-status badge-handoff-cleared">CLEARED_FOR_REUNIFICATION</span>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of ['#test-badge-enroute', '#test-badge-treatment', '#test-badge-stabilized', '#test-badge-cleared']) {
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
