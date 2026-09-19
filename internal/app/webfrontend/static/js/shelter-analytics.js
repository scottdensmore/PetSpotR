// PetSpotR Shelter Partner Analytics Dashboard
// Strict CSP compliant - no inline scripts or eval

(function () {
  'use strict';

  function updateExportLinks(range, shelterId) {
    const params = new URLSearchParams();
    if (range) {
      params.set('range', range);
    }
    if (shelterId) {
      params.set('shelterId', shelterId);
    }
    const qs = params.toString() ? '?' + params.toString() : '';

    const csvBtn = document.getElementById('btn-export-csv');
    if (csvBtn) {
      csvBtn.href = '/api/v1/shelters/analytics/export.csv' + qs;
    }

    const geojsonBtn = document.getElementById('btn-export-geojson');
    if (geojsonBtn) {
      geojsonBtn.href = '/api/v1/shelters/analytics/export.geojson' + qs;
    }
  }

  function populateShelterFilter(availableShelters) {
    const select = document.getElementById('shelter-filter');
    if (!select) return;

    const currentSelection = select.value;
    // Keep the first default option ("All Municipal Shelters")
    while (select.options.length > 1) {
      select.remove(1);
    }

    if (Array.isArray(availableShelters)) {
      availableShelters.forEach((shelter) => {
        if (!shelter || !shelter.id) return;
        const opt = document.createElement('option');
        opt.value = shelter.id;
        opt.textContent = shelter.name ? `${shelter.name} (${shelter.id})` : shelter.id;
        select.appendChild(opt);
      });
    }

    if (currentSelection) {
      select.value = currentSelection;
    }
  }

  function updateKPIs(kpis) {
    const data = kpis || {};

    const rtoEl = document.getElementById('kpi-rto-rate');
    if (rtoEl) {
      const rate = typeof data.returnToOwnerRate === 'number'
        ? (data.returnToOwnerRate * 100).toFixed(1) + '%'
        : '0.0%';
      rtoEl.textContent = rate;
    }

    const turnaroundEl = document.getElementById('kpi-turnaround');
    if (turnaroundEl) {
      const hours = typeof data.medianIntakeToReunionHours === 'number'
        ? data.medianIntakeToReunionHours.toFixed(1) + 'h'
        : '0.0h';
      turnaroundEl.textContent = hours;
    }

    const turnaroundSubtext = document.getElementById('kpi-turnaround-subtext');
    if (turnaroundSubtext) {
      if (typeof data.medianIntakeToMatchHours === 'number' && data.medianIntakeToMatchHours > 0) {
        turnaroundSubtext.textContent = `Match: ${data.medianIntakeToMatchHours.toFixed(1)}h | Reunion Velocity`;
      } else {
        turnaroundSubtext.textContent = 'Intake to Reunion Velocity';
      }
    }

    const microchipEl = document.getElementById('kpi-microchip-rate');
    if (microchipEl) {
      const rate = typeof data.microchipScanRate === 'number'
        ? (data.microchipScanRate * 100).toFixed(1) + '%'
        : '0.0%';
      microchipEl.textContent = rate;
    }

    const ratioEl = document.getElementById('kpi-deterministic-ratio');
    if (ratioEl) {
      const ratio = typeof data.deterministicMatchRatio === 'number'
        ? (data.deterministicMatchRatio * 100).toFixed(1) + '%'
        : '0.0%';
      ratioEl.textContent = ratio;
    }
  }

  function renderBreakdownTable(shelters) {
    const tbody = document.getElementById('shelter-breakdown-body');
    if (!tbody) return;

    while (tbody.firstChild) {
      tbody.removeChild(tbody.firstChild);
    }

    if (!Array.isArray(shelters) || shelters.length === 0) {
      const emptyTr = document.createElement('tr');
      const emptyTd = document.createElement('td');
      emptyTd.colSpan = 6;
      emptyTd.className = 'text-center text-muted';
      emptyTd.textContent = 'No municipal shelter records found for the selected filter.';
      emptyTr.appendChild(emptyTd);
      tbody.appendChild(emptyTr);
      return;
    }

    shelters.forEach((item) => {
      const tr = document.createElement('tr');

      const tdShelter = document.createElement('td');
      const strongName = document.createElement('strong');
      strongName.textContent = item.shelterName || item.shelterId || 'Unknown Shelter';
      tdShelter.appendChild(strongName);
      if (item.shelterId && item.shelterName && item.shelterId !== item.shelterName) {
        const idBadge = document.createElement('span');
        idBadge.className = 'text-secondary text-small';
        idBadge.style.display = 'block';
        idBadge.textContent = item.shelterId;
        tdShelter.appendChild(idBadge);
      }
      tr.appendChild(tdShelter);

      const tdIntakes = document.createElement('td');
      tdIntakes.textContent = (item.intakeCount || 0).toLocaleString();
      tr.appendChild(tdIntakes);

      const tdCare = document.createElement('td');
      tdCare.textContent = (item.activeCareCount || 0).toLocaleString();
      tr.appendChild(tdCare);

      const tdReunited = document.createElement('td');
      tdReunited.textContent = (item.reunitedCount || 0).toLocaleString();
      tr.appendChild(tdReunited);

      const tdRto = document.createElement('td');
      const rtoRate = typeof item.returnToOwnerRate === 'number'
        ? (item.returnToOwnerRate * 100).toFixed(1) + '%'
        : '0.0%';
      tdRto.textContent = rtoRate;
      tr.appendChild(tdRto);

      const tdChip = document.createElement('td');
      const chipRate = typeof item.microchipScanRate === 'number'
        ? (item.microchipScanRate * 100).toFixed(1) + '%'
        : '0.0%';
      tdChip.textContent = chipRate;
      tr.appendChild(tdChip);

      tbody.appendChild(tr);
    });
  }

  async function fetchAnalytics() {
    const container = document.querySelector('.analytics-dashboard') || document.getElementById('main-content');
    if (container) {
      container.setAttribute('aria-busy', 'true');
    }

    const shelterSelect = document.getElementById('shelter-filter');
    const dateSelect = document.getElementById('date-range-filter');

    const range = dateSelect ? dateSelect.value : '30d';
    const shelterId = shelterSelect ? shelterSelect.value : '';

    updateExportLinks(range, shelterId);

    const params = new URLSearchParams();
    if (range) {
      params.set('range', range);
    }
    if (shelterId) {
      params.set('shelterId', shelterId);
    }
    const qs = params.toString() ? '?' + params.toString() : '';

    try {
      const res = await fetch('/api/v1/shelters/analytics' + qs);
      if (!res.ok) {
        throw new Error(`Failed to load shelter analytics (HTTP ${res.status})`);
      }
      const report = await res.json();
      if (report) {
        if (report.availableShelters) {
          populateShelterFilter(report.availableShelters);
        }
        updateKPIs(report.overallKpis);
        renderBreakdownTable(report.shelterBreakdown);
      }
    } catch (err) {
      console.error('Error fetching shelter analytics:', err);

      const rtoEl = document.getElementById('kpi-rto-rate');
      if (rtoEl) {
        rtoEl.textContent = '—';
      }
      const turnaroundEl = document.getElementById('kpi-turnaround');
      if (turnaroundEl) {
        turnaroundEl.textContent = '—';
      }
      const microchipEl = document.getElementById('kpi-microchip-rate');
      if (microchipEl) {
        microchipEl.textContent = '—';
      }
      const ratioEl = document.getElementById('kpi-deterministic-ratio');
      if (ratioEl) {
        ratioEl.textContent = '—';
      }

      const tbody = document.getElementById('shelter-breakdown-body');
      if (tbody) {
        while (tbody.firstChild) {
          tbody.removeChild(tbody.firstChild);
        }
        const errTr = document.createElement('tr');
        const errTd = document.createElement('td');
        errTd.colSpan = 6;
        errTd.className = 'text-center text-error';
        errTd.textContent = 'Unable to load shelter analytics data. Please check your connection and try again.';
        errTr.appendChild(errTd);
        tbody.appendChild(errTr);
      }
    } finally {
      if (container) {
        container.setAttribute('aria-busy', 'false');
      }
    }
  }

  const loadAnalytics = fetchAnalytics;

  document.addEventListener('DOMContentLoaded', () => {
    const shelterSelect = document.getElementById('shelter-filter');
    const dateSelect = document.getElementById('date-range-filter');

    if (shelterSelect) {
      shelterSelect.addEventListener('change', () => {
        fetchAnalytics();
      });
    }

    if (dateSelect) {
      dateSelect.addEventListener('change', () => {
        fetchAnalytics();
      });
    }

    fetchAnalytics();
  });
})();
