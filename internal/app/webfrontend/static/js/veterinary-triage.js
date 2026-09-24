/**
 * PetSpotR Crisis Veterinary Medical Triage Cockpit & Printable Passport Controller
 * Milestone 11.4: Real-time clinical evaluation, species-aware vitals HUD,
 * emergency resuscitation dosing, SSE patient stream, and offline outbox.
 */

(function () {
  'use strict';

  // Constants
  const DB_NAME = 'petspotr_veterinary_db';
  const DB_VERSION = 1;
  const STORE_OUTBOX = 'triage_outbox';

  let dbInstance = null;
  let sseEventSource = null;
  let activeAssessmentId = null;
  let activePetId = null;

  // DOM Elements cache
  let formTriage = null;
  let liveIndicator = null;
  let liveReasons = null;
  let patientStream = null;
  let treatmentPanel = null;
  let treatmentLog = null;
  let formTreatment = null;
  let btnPrintPassport = null;
  let btnPrintTop = null;

  /**
   * Initialize IndexedDB for offline triage outbox
   */
  async function initIndexedDB() {
    if (typeof indexedDB === 'undefined') return null;
    return new Promise((resolve) => {
      const req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = (e) => {
        const db = e.target.result;
        if (!db.objectStoreNames.contains(STORE_OUTBOX)) {
          db.createObjectStore(STORE_OUTBOX, { keyPath: 'id', autoIncrement: true });
        }
      };
      req.onsuccess = (e) => {
        dbInstance = e.target.result;
        resolve(dbInstance);
      };
      req.onerror = () => {
        console.warn('Failed to open triage IndexedDB');
        resolve(null);
      };
    });
  }

  /**
   * Queue pending triage assessment offline
   */
  async function queueOfflineAssessment(payload) {
    if (!dbInstance) await initIndexedDB();
    if (!dbInstance) return;
    return new Promise((resolve) => {
      try {
        const tx = dbInstance.transaction(STORE_OUTBOX, 'readwrite');
        const store = tx.objectStore(STORE_OUTBOX);
        store.add({ payload, createdAt: new Date().toISOString() });
        tx.oncomplete = () => {
          showToast('Offline: Triage assessment queued in local outbox', 'warning');
          resolve();
        };
        tx.onerror = () => resolve();
      } catch (err) {
        console.error('Failed to queue offline assessment:', err);
        resolve();
      }
    });
  }

  /**
   * Drain queued offline triage assessments when connection restores
   */
  async function drainOfflineOutbox() {
    if (!navigator.onLine) return;
    if (!dbInstance) await initIndexedDB();
    if (!dbInstance) return;

    try {
      const tx = dbInstance.transaction(STORE_OUTBOX, 'readonly');
      const store = tx.objectStore(STORE_OUTBOX);
      const req = store.getAll();
      req.onsuccess = async () => {
        const items = req.result || [];
        if (items.length === 0) return;

        for (const item of items) {
          try {
            const res = await fetch('/api/v1/veterinary/triage', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(item.payload),
            });
            if (res.ok) {
              const delTx = dbInstance.transaction(STORE_OUTBOX, 'readwrite');
              delTx.objectStore(STORE_OUTBOX).delete(item.id);
            }
          } catch (err) {
            console.warn('Outbox sync retry failed:', err);
            break;
          }
        }
      };
    } catch (err) {
      console.error('Error draining triage outbox:', err);
    }
  }

  /**
   * Species-aware clinical triage acuity calculation
   */
  function evaluateTriage(species, vitals, trauma) {
    const reasons = [];
    const isCat = (species || '').toLowerCase() === 'cat' || (species || '').toLowerCase() === 'feline';

    const hasLivingTrauma = trauma.arterialHemorrhage ||
      trauma.penetratingChest ||
      trauma.severeBurns ||
      trauma.moderateBurns ||
      trauma.openFracture;

    // 1. Black: Deceased / Expectant
    if (trauma.unresponsiveAsystole || (!hasLivingTrauma && vitals.heartRateBpm === 0 && vitals.respiratoryRateBpm === 0 && vitals.glasgowComaScale === 3)) {
      return { category: 'TRIAGE_BLACK', reasons: ['Absence of heartbeat, respiration, and cortical responsiveness'] };
    }

    // 2. Red: Immediate / Life-Threatening
    if (trauma.arterialHemorrhage) reasons.push('Active arterial hemorrhage');
    if (trauma.penetratingChest) reasons.push('Penetrating thoracic injury / open pneumothorax');
    if (trauma.severeBurns) reasons.push('Extensive burns (> 30% body surface area)');
    if (vitals.mucousMembrane === 'MM_CYANOTIC') reasons.push('Cyanotic mucous membranes (severe hypoxia)');
    if (vitals.mucousMembrane === 'MM_BRICK_RED') reasons.push('Brick-red mucous membranes (severe sepsis / hyperdynamic shock)');
    if (vitals.capillaryRefillSec >= 3.0) reasons.push(`Critical hypoperfusion (CRT ${vitals.capillaryRefillSec.toFixed(1)}s >= 3.0s)`);
    if (vitals.glasgowComaScale > 0 && vitals.glasgowComaScale <= 8) reasons.push(`Severe mentation impairment (GCS ${vitals.glasgowComaScale} <= 8)`);

    const vitalsRecorded = vitals.heartRateBpm > 0 ||
      vitals.respiratoryRateBpm > 0 ||
      vitals.temperatureF > 0 ||
      vitals.capillaryRefillSec > 0 ||
      Boolean(vitals.mucousMembrane) ||
      vitals.glasgowComaScale > 0;

    if (vitalsRecorded) {
      if (vitals.heartRateBpm === 0) reasons.push('Absent heart rate / pulseless arrest');
      if (vitals.respiratoryRateBpm === 0) reasons.push('Apnea / respiratory arrest');
    }

    // Species-specific thresholds
    if (isCat) {
      if (vitals.heartRateBpm > 0 && (vitals.heartRateBpm < 100 || vitals.heartRateBpm > 260)) {
        reasons.push(`Feline critical heart rate (${vitals.heartRateBpm} BPM)`);
      }
      if (vitals.respiratoryRateBpm > 0 && (vitals.respiratoryRateBpm < 12 || vitals.respiratoryRateBpm > 80)) {
        reasons.push(`Feline critical respiratory rate (${vitals.respiratoryRateBpm} BPM)`);
      }
    } else {
      if (vitals.heartRateBpm > 0 && (vitals.heartRateBpm < 50 || vitals.heartRateBpm > 220)) {
        reasons.push(`Canine critical heart rate (${vitals.heartRateBpm} BPM)`);
      }
      if (vitals.respiratoryRateBpm > 0 && (vitals.respiratoryRateBpm < 8 || vitals.respiratoryRateBpm > 60)) {
        reasons.push(`Canine critical respiratory rate (${vitals.respiratoryRateBpm} BPM)`);
      }
    }

    if (vitals.temperatureF > 0 && (vitals.temperatureF < 96.0 || vitals.temperatureF > 105.0)) {
      reasons.push(`Critical body temperature (${vitals.temperatureF.toFixed(1)} °F)`);
    }

    if (reasons.length > 0) {
      return { category: 'TRIAGE_RED', reasons };
    }

    // 3. Yellow: Delayed / Serious
    if (trauma.openFracture) reasons.push('Open skeletal fracture without active arterial bleeding');
    if (trauma.moderateBurns) reasons.push('Moderate thermal burns (10-30% body surface area)');
    if (vitals.capillaryRefillSec >= 2.0) reasons.push(`Elevated CRT (${vitals.capillaryRefillSec.toFixed(1)}s)`);
    if (vitals.mucousMembrane === 'MM_PALE') reasons.push('Pale mucous membranes (early shock / blood loss)');
    if (vitals.mucousMembrane === 'MM_ICTERIC') reasons.push('Icteric mucous membranes (jaundice / hepatic or hemolytic crisis)');
    if (vitals.glasgowComaScale >= 9 && vitals.glasgowComaScale <= 14) reasons.push(`Depressed mentation (GCS ${vitals.glasgowComaScale})`);
    if (vitals.temperatureF > 0 && (vitals.temperatureF < 99.5 || vitals.temperatureF > 103.5)) {
      reasons.push(`Abnormal temperature (${vitals.temperatureF.toFixed(1)} °F)`);
    }

    if (reasons.length > 0) {
      return { category: 'TRIAGE_YELLOW', reasons };
    }

    // 4. Green: Minor / Walking Wounded
    return { category: 'TRIAGE_GREEN', reasons: ['Stable physiological vitals within baseline bounds'] };
  }

  /**
   * Weight-adjusted resuscitation dosages
   */
  function calculateDosages(species, weightKg) {
    const w = weightKg > 0 ? weightKg : 10.0;
    const isCat = (species || '').toLowerCase() === 'cat' || (species || '').toLowerCase() === 'feline';
    const fluidMl = isCat ? w * 7.5 : w * 15.0;
    const epiMg = w * 0.01;
    const bupMg = w * 0.02;

    return {
      fluids: `${Math.round(fluidMl)} mL IV over 15 min`,
      epinephrine: `${epiMg.toFixed(2)} mg (${epiMg.toFixed(2)} mL of 1:1000) IV/IO`,
      buprenorphine: `${bupMg.toFixed(2)} mg (${(bupMg / 0.3).toFixed(2)} mL of 0.3mg/mL) IV/IM/SL`,
    };
  }

  /**
   * Recalculate and update the live triage indicator and emergency dosage bar
   */
  function calculateLiveTriage() {
    if (!liveIndicator) return;

    const species = document.getElementById('triage-species')?.value || 'Dog';
    const weightVal = parseFloat(document.getElementById('triage-weight')?.value) || 10.0;
    const hrVal = parseInt(document.getElementById('vitals-hr')?.value, 10) || 0;
    const rrVal = parseInt(document.getElementById('vitals-rr')?.value, 10) || 0;
    const tempVal = parseFloat(document.getElementById('vitals-temp')?.value) || 0;
    const crtVal = parseFloat(document.getElementById('vitals-crt')?.value) || 1.5;
    const mmVal = document.getElementById('vitals-mm')?.value || 'MM_PINK';
    const gcsVal = parseInt(document.getElementById('vitals-gcs')?.value, 10) || 18;

    const trauma = {
      arterialHemorrhage: Boolean(document.getElementById('trauma-arterial-hemorrhage')?.checked),
      penetratingChest: Boolean(document.getElementById('trauma-penetrating-chest')?.checked),
      severeBurns: Boolean(document.getElementById('trauma-severe-burns')?.checked),
      moderateBurns: Boolean(document.getElementById('trauma-moderate-burns')?.checked),
      openFracture: Boolean(document.getElementById('trauma-open-fracture')?.checked),
      unresponsiveAsystole: Boolean(document.getElementById('trauma-unresponsive-asystole')?.checked),
    };

    const vitals = {
      heartRateBpm: hrVal,
      respiratoryRateBpm: rrVal,
      temperatureF: tempVal,
      capillaryRefillSec: crtVal,
      mucousMembrane: mmVal,
      glasgowComaScale: gcsVal,
    };

    const { category, reasons } = evaluateTriage(species, vitals, trauma);

    // Update Live Indicator Class and Text
    liveIndicator.classList.remove('badge-triage-red', 'badge-triage-yellow', 'badge-triage-green', 'badge-triage-black');
    const colorClass = category.toLowerCase().replace('_', '-');
    liveIndicator.classList.add(`badge-${colorClass}`);
    liveIndicator.textContent = category.replace('_', ' ');

    // Update Reasons list
    if (liveReasons) {
      liveReasons.innerHTML = '';
      reasons.forEach((r) => {
        const li = document.createElement('li');
        li.textContent = r;
        liveReasons.appendChild(li);
      });
    }

    // Update Emergency Dosages
    const dosages = calculateDosages(species, weightVal);
    const fluidsEl = document.getElementById('dosage-fluids');
    if (fluidsEl) fluidsEl.textContent = dosages.fluids;
    const epiEl = document.getElementById('dosage-epinephrine');
    if (epiEl) epiEl.textContent = dosages.epinephrine;
    const bupEl = document.getElementById('dosage-buprenorphine');
    if (bupEl) bupEl.textContent = dosages.buprenorphine;
  }

  /**
   * Helper to display temporary toast notifications
   */
  function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    if (!container) return;
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => {
      toast.remove();
    }, 4000);
  }

  /**
   * Add a patient card to the stream
   */
  function addOrUpdatePatientCard(assessment) {
    if (!patientStream) return;

    const emptyMsg = document.getElementById('stream-empty-msg');
    if (emptyMsg) emptyMsg.remove();

    let existingCard = patientStream.querySelector(`.patient-card[data-pet-id="${assessment.petId}"]`);
    if (!existingCard) {
      existingCard = patientStream.querySelector(`.patient-card[data-assessment-id="${assessment.assessmentId}"]`);
    }

    const badgeClass = `badge-${assessment.category.toLowerCase().replace('_', '-')}`;
    const cardHtml = `
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
        <span class="badge ${badgeClass}">${assessment.category.replace('_', ' ')}</span>
        <span style="font-weight: 700; font-size: 0.875rem;">${assessment.petId}</span>
      </div>
      <div style="font-size: 0.8125rem; color: var(--text-secondary);">
        <span>${assessment.species} (${(assessment.weightKg || 0).toFixed(1)} kg)</span> • 
        <span>HR: ${assessment.vitals?.heartRateBpm || 0} | RR: ${assessment.vitals?.respiratoryRateBpm || 0}</span>
      </div>
      <div style="margin-top: 0.5rem; display: flex; gap: 0.5rem;">
        <a href="/p/${assessment.petId}/passport" class="btn btn-xs btn-secondary" style="font-size: 0.75rem; padding: 0.2rem 0.5rem;">View Passport</a>
      </div>
    `;

    if (existingCard) {
      existingCard.setAttribute('data-pet-id', assessment.petId);
      existingCard.setAttribute('data-assessment-id', assessment.assessmentId);
      existingCard.innerHTML = cardHtml;
    } else {
      const newCard = document.createElement('article');
      newCard.className = 'patient-card';
      newCard.setAttribute('data-pet-id', assessment.petId);
      newCard.setAttribute('data-assessment-id', assessment.assessmentId);
      newCard.setAttribute('tabindex', '0');
      newCard.setAttribute('role', 'article');
      newCard.innerHTML = cardHtml;
      patientStream.prepend(newCard);
      attachCardClick(newCard);
    }

    updatePatientCount();
  }

  function updatePatientCount() {
    if (!patientStream) return;
    const cards = patientStream.querySelectorAll('.patient-card');
    const badge = document.getElementById('patient-count-badge');
    if (badge) {
      badge.textContent = `${cards.length} Patients`;
    }
  }

  /**
   * Handle card click to open treatments
   */
  function attachCardClick(card) {
    card.addEventListener('click', () => {
      const petId = card.getAttribute('data-pet-id');
      const assessmentId = card.getAttribute('data-assessment-id');
      selectPatient(petId, assessmentId);
    });
  }

  async function selectPatient(petId, assessmentId) {
    activePetId = petId;
    activeAssessmentId = assessmentId;

    if (!treatmentPanel) return;

    // Highlight card
    document.querySelectorAll('.patient-card').forEach((c) => {
      c.classList.toggle('selected', c.getAttribute('data-pet-id') === petId);
    });

    treatmentPanel.hidden = false;
    const titleEl = document.getElementById('treatment-patient-title');
    if (titleEl) titleEl.textContent = `Patient ${petId} Treatments`;

    await loadTreatments(petId, assessmentId);
  }

  async function loadTreatments(petId, assessmentId) {
    if (!treatmentLog) return;
    treatmentLog.innerHTML = '<p class="text-secondary text-xs">Loading treatments...</p>';

    try {
      const res = await fetch(`/api/v1/veterinary/triage/${encodeURIComponent(petId)}`);
      if (res.ok) {
        const assessments = await res.json();
        const assessment = assessments.find((a) => a.assessmentId === assessmentId || a.petId === petId) || assessments[0];
        renderTreatments(assessment?.administeredTreatments || []);
      } else {
        renderTreatments([]);
      }
    } catch (err) {
      console.error('Failed to load treatments:', err);
      renderTreatments([]);
    }
  }

  function renderTreatments(treatments) {
    if (!treatmentLog) return;
    if (!treatments || treatments.length === 0) {
      treatmentLog.innerHTML = '<p class="text-secondary text-xs">No emergency treatments recorded yet.</p>';
      return;
    }

    treatmentLog.innerHTML = '';
    treatments.forEach((t) => {
      const div = document.createElement('div');
      div.className = 'treatment-entry';
      const timeStr = t.administeredAt ? new Date(t.administeredAt).toLocaleTimeString() : '';
      div.innerHTML = `
        <div style="display: flex; justify-content: space-between; font-weight: 700;">
          <span>${t.medicationName} — ${t.dosage} (${t.route})</span>
          <span style="font-size: 0.75rem; color: var(--text-muted);">${timeStr}</span>
        </div>
        <div style="font-size: 0.8125rem; color: var(--text-secondary); margin-top: 0.25rem;">
          By: ${t.administeredBy || 'Medic'} ${t.notes ? `• ${t.notes}` : ''}
        </div>
      `;
      treatmentLog.appendChild(div);
    });
  }

  /**
   * Handle form triage submission
   */
  async function handleSubmitTriage(e) {
    e.preventDefault();

    const petId = document.getElementById('triage-pet-id')?.value?.trim();
    if (!petId) {
      showToast('Pet ID is required', 'error');
      document.getElementById('triage-pet-id')?.focus();
      return;
    }

    const species = document.getElementById('triage-species')?.value || 'Dog';
    const weightVal = parseFloat(document.getElementById('triage-weight')?.value) || 10.0;
    const hrVal = parseInt(document.getElementById('vitals-hr')?.value, 10) || 0;
    const rrVal = parseInt(document.getElementById('vitals-rr')?.value, 10) || 0;
    const tempVal = parseFloat(document.getElementById('vitals-temp')?.value) || 0;
    const crtVal = parseFloat(document.getElementById('vitals-crt')?.value) || 1.5;
    const mmVal = document.getElementById('vitals-mm')?.value || 'MM_PINK';
    const gcsVal = parseInt(document.getElementById('vitals-gcs')?.value, 10) || 18;

    const hubId = document.getElementById('triage-hub-id')?.value?.trim() || '';
    const medicName = document.getElementById('triage-medic-name')?.value?.trim() || 'Field Medic';
    const traumaNotes = document.getElementById('triage-notes')?.value?.trim() || '';

    const trauma = {
      arterialHemorrhage: Boolean(document.getElementById('trauma-arterial-hemorrhage')?.checked),
      penetratingChest: Boolean(document.getElementById('trauma-penetrating-chest')?.checked),
      severeBurns: Boolean(document.getElementById('trauma-severe-burns')?.checked),
      moderateBurns: Boolean(document.getElementById('trauma-moderate-burns')?.checked),
      openFracture: Boolean(document.getElementById('trauma-open-fracture')?.checked),
      unresponsiveAsystole: Boolean(document.getElementById('trauma-unresponsive-asystole')?.checked),
    };

    const vitals = {
      heartRateBpm: hrVal,
      respiratoryRateBpm: rrVal,
      temperatureF: tempVal,
      capillaryRefillSec: crtVal,
      mucousMembrane: mmVal,
      glasgowComaScale: gcsVal,
    };

    const payload = {
      petId,
      hubId,
      medicId: 'medic-field-01',
      medicName,
      species,
      weightKg: weightVal,
      vitals,
      trauma,
      traumaNotes,
    };

    if (!navigator.onLine) {
      await queueOfflineAssessment(payload);
      addOrUpdatePatientCard({
        assessmentId: `offline-${Date.now()}`,
        petId,
        species,
        weightKg: weightVal,
        category: evaluateTriage(species, vitals, trauma).category,
        vitals,
      });
      return;
    }

    try {
      const res = await fetch('/api/v1/veterinary/triage', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        throw new Error(`HTTP error ${res.status}`);
      }

      const assessment = await res.json();
      showToast(`Triage assessment recorded for ${petId} (${assessment.category})`, 'success');
      addOrUpdatePatientCard(assessment);
      selectPatient(assessment.petId, assessment.assessmentId);
    } catch (err) {
      console.warn('Network error recording assessment, queueing locally:', err);
      await queueOfflineAssessment(payload);
      addOrUpdatePatientCard({
        assessmentId: `offline-${Date.now()}`,
        petId,
        species,
        weightKg: weightVal,
        category: evaluateTriage(species, vitals, trauma).category,
        vitals,
      });
    }
  }

  /**
   * Handle treatment submission
   */
  async function handleSubmitTreatment(e) {
    e.preventDefault();
    if (!activeAssessmentId) {
      showToast('No active patient assessment selected', 'warning');
      return;
    }

    const medName = document.getElementById('treatment-medication')?.value?.trim();
    const dosage = document.getElementById('treatment-dosage')?.value?.trim();
    const route = document.getElementById('treatment-route')?.value || 'IV';
    const adminBy = document.getElementById('treatment-admin-by')?.value?.trim() || 'Field Medic';
    const notes = document.getElementById('treatment-notes')?.value?.trim() || '';

    if (!medName || !dosage) {
      showToast('Medication and dosage are required', 'error');
      return;
    }

    try {
      const res = await fetch(`/api/v1/veterinary/triage/${encodeURIComponent(activeAssessmentId)}/treatments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          medicationName: medName,
          dosage,
          route,
          administeredBy: adminBy,
          notes,
        }),
      });

      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      const updated = await res.json();
      showToast(`Treatment administered: ${medName}`, 'success');
      renderTreatments(updated.administeredTreatments || []);
      formTreatment.reset();
    } catch (err) {
      console.error('Failed to administer treatment:', err);
      showToast('Failed to record treatment', 'error');
    }
  }

  /**
   * SSE stream listener for real-time triage updates
   */
  function setupSSE() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseEventSource = new EventSource('/api/v1/reunions/events?matchId=triage');
      sseEventSource.onmessage = (e) => {
        try {
          const event = JSON.parse(e.data);
          if (event.type === 'triage_assessment_created' && event.payload) {
            addOrUpdatePatientCard({
              assessmentId: event.payload.assessmentId,
              petId: event.payload.petId,
              species: event.payload.species,
              category: event.payload.category,
              weightKg: event.payload.weightKg || 10,
              vitals: event.payload.vitals || {},
            });
          } else if (event.type === 'triage_treatment_administered' && event.payload) {
            if (activeAssessmentId === event.payload.assessmentId) {
              loadTreatments(activePetId, activeAssessmentId);
            }
          }
        } catch (err) {
          // ignore heartbeat / unparseable
        }
      };
    } catch (err) {
      console.warn('Could not establish triage SSE stream:', err);
    }
  }

  /**
   * DOM Initializer
   */
  function init() {
    formTriage = document.getElementById('form-triage');
    liveIndicator = document.getElementById('live-triage-indicator');
    liveReasons = document.getElementById('live-triage-reasons');
    patientStream = document.getElementById('patient-triage-stream');
    treatmentPanel = document.getElementById('patient-treatment-panel');
    treatmentLog = document.getElementById('treatment-log');
    formTreatment = document.getElementById('form-administer-treatment');
    btnPrintPassport = document.getElementById('btn-print-passport');
    btnPrintTop = document.getElementById('btn-print-top');

    // Print handlers
    if (btnPrintPassport) {
      btnPrintPassport.addEventListener('click', () => window.print());
    }
    if (btnPrintTop) {
      btnPrintTop.addEventListener('click', () => window.print());
    }

    if (formTriage) {
      // Species buttons
      const btnDog = document.getElementById('triage-species-dog');
      const btnCat = document.getElementById('triage-species-cat');
      const inputSpecies = document.getElementById('triage-species');
      const hrHint = document.getElementById('hr-range-hint');
      const rrHint = document.getElementById('rr-range-hint');

      if (btnDog && btnCat) {
        btnDog.addEventListener('click', () => {
          btnDog.classList.add('active');
          btnDog.setAttribute('aria-pressed', 'true');
          btnCat.classList.remove('active');
          btnCat.setAttribute('aria-pressed', 'false');
          if (inputSpecies) inputSpecies.value = 'Dog';
          if (hrHint) hrHint.textContent = 'Canine ref: 60-140 BPM';
          if (rrHint) rrHint.textContent = 'Canine ref: 10-30 BPM';
          calculateLiveTriage();
        });

        btnCat.addEventListener('click', () => {
          btnCat.classList.add('active');
          btnCat.setAttribute('aria-pressed', 'true');
          btnDog.classList.remove('active');
          btnDog.setAttribute('aria-pressed', 'false');
          if (inputSpecies) inputSpecies.value = 'Cat';
          if (hrHint) hrHint.textContent = 'Feline ref: 140-220 BPM';
          if (rrHint) rrHint.textContent = 'Feline ref: 20-30 BPM';
          calculateLiveTriage();
        });
      }

      // CRT buttons
      const crtButtons = document.querySelectorAll('.choice-group [data-crt]');
      const crtInput = document.getElementById('vitals-crt');
      crtButtons.forEach((btn) => {
        btn.addEventListener('click', () => {
          crtButtons.forEach((b) => b.classList.remove('active'));
          btn.classList.add('active');
          if (crtInput) crtInput.value = btn.getAttribute('data-crt');
          calculateLiveTriage();
        });
      });

      // Mucous Membrane buttons
      const mmButtons = document.querySelectorAll('.choice-group [data-mm]');
      const mmInput = document.getElementById('vitals-mm');
      mmButtons.forEach((btn) => {
        btn.addEventListener('click', () => {
          mmButtons.forEach((b) => b.classList.remove('active'));
          btn.classList.add('active');
          if (mmInput) mmInput.value = btn.getAttribute('data-mm');
          calculateLiveTriage();
        });
      });

      // Vitals & trauma inputs triggers
      ['vitals-hr', 'vitals-rr', 'vitals-temp', 'vitals-gcs', 'triage-weight'].forEach((id) => {
        const el = document.getElementById(id);
        if (el) {
          el.addEventListener('input', calculateLiveTriage);
          el.addEventListener('change', calculateLiveTriage);
        }
      });

      [
        'trauma-arterial-hemorrhage',
        'trauma-penetrating-chest',
        'trauma-severe-burns',
        'trauma-moderate-burns',
        'trauma-open-fracture',
        'trauma-unresponsive-asystole',
      ].forEach((id) => {
        const el = document.getElementById(id);
        if (el) el.addEventListener('change', calculateLiveTriage);
      });

      formTriage.addEventListener('submit', handleSubmitTriage);
      formTriage.addEventListener('reset', () => {
        setTimeout(calculateLiveTriage, 50);
      });

      // Attach clicks to existing SSR patient cards
      if (patientStream) {
        patientStream.querySelectorAll('.patient-card').forEach((card) => {
          attachCardClick(card);
        });
        updatePatientCount();
      }

      // Stream filter buttons
      document.querySelectorAll('[data-stream-filter]').forEach((btn) => {
        btn.addEventListener('click', () => {
          document.querySelectorAll('[data-stream-filter]').forEach((b) => b.classList.remove('active'));
          btn.classList.add('active');
          const filter = btn.getAttribute('data-stream-filter');
          if (!patientStream) return;
          patientStream.querySelectorAll('.patient-card').forEach((card) => {
            if (filter === 'ALL') {
              card.style.display = '';
            } else {
              const badge = card.querySelector('.badge');
              const text = badge ? badge.textContent.trim().replace(' ', '_') : '';
              card.style.display = text === filter ? '' : 'none';
            }
          });
        });
      });

      if (formTreatment) {
        formTreatment.addEventListener('submit', handleSubmitTreatment);
      }

      // Initial live triage calculation
      calculateLiveTriage();

      // Setup SSE & Outbox
      setupSSE();
      initIndexedDB();
      window.addEventListener('online', drainOfflineOutbox);
    }
  }

  // Self-initialize on DOM ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
