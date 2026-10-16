import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Milestone 12.2: Veterinary Clinic & Shelter HL7 FHIR / PIMS Exchange Journey', () => {
  test('Complete 6-Step Veterinary HL7 FHIR R4 & PIMS Clinical Handoff User Journey', async ({ page, request }) => {
    const petId = 'pet-fhir-e2e-101';
    const clinicName = 'Puget Sound Veterinary Trauma Center';
    let assessmentId = '';
    let registeredClinicId = '';
    let handoffId = '';

    // -------------------------------------------------------------------------
    // Step 1: Issue Emergency Veterinary Passport & Record Critical Field Triage Assessment
    // -------------------------------------------------------------------------
    await test.step('Step 1: Issue Emergency Veterinary Passport & Record Critical Field Triage Assessment', async () => {
      const passportRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/passports`, {
        data: {
          petId,
          petName: 'Apollo',
          species: 'Dog',
          breed: 'German Shepherd',
          microchipId: '985141009988776',
          rabiesTagID: 'RAB-2026-APOLLO',
          weightKg: 36.0,
          vaccinations: [
            {
              vaccineName: 'Rabies',
              expirationDate: '2028-10-16T00:00:00Z',
              verified: true,
            },
          ],
          allergies: [
            {
              allergen: 'Penicillin',
              severity: 'ANAPHYLACTIC',
              reactionDescription: 'Acute anaphylactic shock',
            },
          ],
        },
      });
      expect(passportRes.ok()).toBeTruthy();
      const passportBody = await passportRes.json();
      expect(passportBody.passportId).toBeTruthy();

      const triageRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/triage`, {
        data: {
          petId,
          hubId: 'hub-seattle-center',
          medicId: 'medic-sar-01',
          medicName: 'Dr. Aris Thorne',
          species: 'Dog',
          weightKg: 36.0,
          vitals: {
            heartRateBpm: 195,
            respiratoryRateBpm: 48,
            temperatureF: 104.2,
            capillaryRefillSec: 3.2,
            mucousMembrane: 'MM_PALE',
            glasgowComaScale: 11,
          },
          trauma: {
            arterialHemorrhage: true,
          },
          traumaNotes: 'Active arterial hemorrhage stabilized with pressure dressing; severe hypovolemic shock',
        },
      });
      expect(triageRes.ok()).toBeTruthy();
      const triageBody = await triageRes.json();
      expect(triageBody.assessmentId).toBeTruthy();
      expect(triageBody.category).toBe('TRIAGE_RED');
      assessmentId = triageBody.assessmentId;

      const treatmentRes = await request.post(
        `${WEB_FRONTEND_URL}/api/v1/veterinary/triage/${encodeURIComponent(assessmentId)}/treatments`,
        {
          data: {
            medicationName: 'Lactated Ringers Bolus',
            dosage: '900 mL',
            route: 'IV',
            administeredBy: 'Dr. Aris Thorne',
            notes: 'Rapid IV crystalloid shock bolus',
          },
        }
      );
      expect(treatmentRes.ok()).toBeTruthy();
    });

    // -------------------------------------------------------------------------
    // Step 2: Navigate to /triage and Follow ER FHIR Handoff Action to /veterinary/fhir
    // -------------------------------------------------------------------------
    await test.step('Step 2: Navigate to /triage and Follow ER FHIR Handoff Action to /veterinary/fhir', async () => {
      await page.goto(`${WEB_FRONTEND_URL}/triage`);
      await page.waitForLoadState('networkidle');

      const patientCard = page.locator(`.patient-card[data-pet-id="${petId}"]`).first();
      await expect(patientCard).toBeVisible();
      await expect(patientCard.locator('.badge-triage-red')).toBeVisible();

      const handoffLink = patientCard.locator('a', { hasText: 'ER FHIR Handoff' });
      await expect(handoffLink).toBeVisible();
      await handoffLink.click();
      await page.waitForLoadState('networkidle');

      await expect(page).toHaveURL(/\/veterinary\/fhir/);
      await expect(page.locator('#handoff-pet-id')).toHaveValue(petId);

      const seededClinics = page.locator('#clinics-list .clinic-partner-card');
      await expect(seededClinics.first()).toBeVisible();
      expect(await seededClinics.count()).toBeGreaterThanOrEqual(3);
    });

    // -------------------------------------------------------------------------
    // Step 3: Register a Specialty Veterinary ER Partner Clinic
    // -------------------------------------------------------------------------
    await test.step('Step 3: Register a Specialty Veterinary ER Partner Clinic', async () => {
      await page.locator('#clinic-name').fill(clinicName);
      await page.locator('#clinic-pims').selectOption('IDEXX_CORNERSTONE');
      await page.locator('#btn-register-clinic').click();

      const registeredClinicCard = page.locator('#clinics-list .clinic-partner-card', { hasText: clinicName }).first();
      await expect(registeredClinicCard).toBeVisible();
      await expect(registeredClinicCard.locator('.badge-pims-provider')).toContainText('IDEXX_CORNERSTONE');

      const clinicIdAttr = await registeredClinicCard.getAttribute('data-clinic-id');
      expect(clinicIdAttr).toBeTruthy();
      registeredClinicId = clinicIdAttr!;

      const clinicSelectOption = page.locator(`#handoff-clinic-select option[value="${registeredClinicId}"]`);
      await expect(clinicSelectOption).toContainText(clinicName);
    });

    // -------------------------------------------------------------------------
    // Step 4: Dispatch Outbound Pre-Arrival ER Handoff & Inspect HL7 FHIR R4 Bundle
    // -------------------------------------------------------------------------
    await test.step('Step 4: Dispatch Outbound Pre-Arrival ER Handoff & Inspect HL7 FHIR R4 Bundle', async () => {
      await page.locator('#handoff-pet-id').fill(petId);
      await page.locator('#handoff-clinic-select').selectOption(registeredClinicId);
      await page.locator('#handoff-eta-minutes').fill('10');
      await page.locator('#btn-dispatch-handoff').click();

      const handoffCard = page.locator('#handoffs-list .handoff-card', { hasText: petId }).first();
      await expect(handoffCard).toBeVisible();
      await expect(handoffCard).toHaveAttribute('data-handoff-status', 'EN_ROUTE_PRE_ARRIVAL');
      await expect(handoffCard.locator('.badge-handoff-enroute')).toContainText('EN_ROUTE_PRE_ARRIVAL');

      const handoffIdAttr = await handoffCard.getAttribute('data-handoff-id');
      expect(handoffIdAttr).toBeTruthy();
      handoffId = handoffIdAttr!;

      await handoffCard.locator('.btn-inspect-fhir').click();

      const fhirBundleViewer = page.locator('#fhir-bundle-json');
      await expect(fhirBundleViewer).toContainText('"resourceType": "Bundle"');
      await expect(fhirBundleViewer).toContainText('http://hl7.org/fhir/StructureDefinition/patient-animal');
      await expect(fhirBundleViewer).toContainText('Penicillin');
      await expect(fhirBundleViewer).toContainText('Lactated Ringers Bolus');
    });

    // -------------------------------------------------------------------------
    // Step 5: Verify Raw FHIR R4 Bundle & PIMS Intake CSV Export Endpoints
    // -------------------------------------------------------------------------
    await test.step('Step 5: Verify Raw FHIR R4 Bundle & PIMS Intake CSV Export Endpoints', async () => {
      const fhirRes = await request.get(
        `${WEB_FRONTEND_URL}/api/v1/veterinary/handoffs/${encodeURIComponent(handoffId)}/fhir`
      );
      expect(fhirRes.status()).toBe(200);
      expect(fhirRes.headers()['content-type']).toContain('application/fhir+json');
      const rawBundle = await fhirRes.json();
      expect(rawBundle.resourceType).toBe('Bundle');
      expect(rawBundle.type).toBe('document');

      const csvRes = await request.get(
        `${WEB_FRONTEND_URL}/api/v1/veterinary/handoffs/${encodeURIComponent(handoffId)}/fhir?format=csv`
      );
      expect(csvRes.status()).toBe(200);
      expect(csvRes.headers()['content-type']).toContain('text/csv');
      const csvText = await csvRes.text();
      expect(csvText).toContain(petId);
      expect(csvText).toContain('Apollo');
      expect(csvText).toContain('IDEXX_CORNERSTONE');
      expect(csvText).toContain('Penicillin (ANAPHYLACTIC)');
    });

    // -------------------------------------------------------------------------
    // Step 6: Transmit Inbound Clinic PIMS Discharge Webhook & Verify Reunification Clearance
    // -------------------------------------------------------------------------
    await test.step('Step 6: Transmit Inbound Clinic PIMS Discharge Webhook & Verify Reunification Clearance', async () => {
      await expect(page.locator('#pims-sim-handoff-id')).toHaveValue(handoffId);
      await page.locator('#pims-sim-status').selectOption('CLEARED_FOR_REUNIFICATION');
      await page.locator('#pims-sim-microchip').fill('985141009988776');
      await page.locator('#pims-sim-vet').fill('Dr. Sarah Jenkins, DACVECC');
      await page.locator('#btn-send-pims-webhook').click();

      const handoffCard = page.locator(`#handoffs-list .handoff-card[data-handoff-id="${handoffId}"]`);
      await expect(handoffCard).toHaveAttribute('data-handoff-status', 'CLEARED_FOR_REUNIFICATION');
      await expect(handoffCard.locator('.badge-handoff-cleared')).toContainText('CLEARED_FOR_REUNIFICATION');
      await expect(handoffCard.locator('.reunion-clearance-banner')).toContainText('985141009988776');
    });
  });
});
