// Client-side controller for Pet Match Comparison Dashboard
document.addEventListener('DOMContentLoaded', () => {
  const container = document.getElementById('matches-list-container');
  const scoreFilter = document.getElementById('scoreFilter');
  const zoomModal = document.getElementById('zoom-modal');
  const zoomedImage = document.getElementById('zoomed-image');

  let allMatches = [];
  let identityEnabled = false;
  let matchLoadRevision = 0;
  let lastIdentityState = '';
  const matchStatuses = new Set(['PENDING_REVIEW', 'CONFIRMED', 'REJECTED', 'REUNITED']);
  const allowedImageHosts = new Set(['storage.petspotr.io']);

  function openModal(modal) {
    if (modal) modal.hidden = false;
  }

  function closeModal(modal) {
    if (modal) modal.hidden = true;
  }

  function createElement(tagName, options = {}) {
    const element = document.createElement(tagName);
    if (options.className) element.className = options.className;
    if (options.text !== undefined) element.textContent = options.text;
    return element;
  }

  function replaceMatchState(message, className) {
    allMatches = [];
    matchLoadRevision += 1;
    if (!container) return;
    if (!message) {
      container.replaceChildren();
      return;
    }
    const stateMessage = createElement('p', { text: message, className });
    stateMessage.setAttribute('role', 'status');
    container.replaceChildren(stateMessage);
  }

  function validString(value, allowEmpty = false) {
    return typeof value === 'string' && (allowEmpty || value.trim().length > 0);
  }

  function validRecordId(value) {
    return validString(value);
  }

  function validScore(value) {
    return typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 1;
  }

  function trustedImageURL(value) {
    if (typeof value !== 'string' || value.length === 0 || value.length > 2048) return null;

    let parsed;
    try {
      parsed = new URL(value, window.location.origin);
    } catch (_) {
      return null;
    }

    if (parsed.username || parsed.password) return null;
    if (parsed.origin === window.location.origin && (parsed.protocol === 'http:' || parsed.protocol === 'https:')) {
      return parsed.href;
    }
    if (parsed.protocol !== 'https:') return null;
    if (allowedImageHosts.has(parsed.hostname)) return parsed.href;
    return null;
  }

  function normalizePet(value, requireName) {
    if (!value || typeof value !== 'object' || !validRecordId(value.petId)) return null;
    const petName = value.petName ?? '';
    if (!validString(petName, !requireName) || !validString(value.breed) || !validString(value.location)) {
      return null;
    }

    return {
      petId: value.petId,
      petName,
      breed: value.breed,
      location: value.location,
      imageUrl: trustedImageURL(value.imageUrl),
    };
  }

  function normalizeMatch(value) {
    if (!value || typeof value !== 'object') return null;
    if (!validRecordId(value.matchId) || !validRecordId(value.foundPetId) || !validRecordId(value.matchedPetId)) {
      return null;
    }
    if (!validScore(value.score) || !matchStatuses.has(value.status)) return null;
    if (!value.scores || typeof value.scores !== 'object') return null;
    if (!validScore(value.scores.visual) || !validScore(value.scores.color) || !validScore(value.scores.spatial)) {
      return null;
    }
    if (typeof value.scores.distanceMiles !== 'number' || !Number.isFinite(value.scores.distanceMiles) ||
        value.scores.distanceMiles < 0 || value.scores.distanceMiles > 25000) {
      return null;
    }

    if (typeof value.matchedAt !== 'string' || value.matchedAt.length > 64) return null;
    const matchedAt = new Date(value.matchedAt);
    if (Number.isNaN(matchedAt.getTime())) return null;
    const lostPet = normalizePet(value.lostPet, true);
    const foundPet = normalizePet(value.foundPet, false);
    if (!lostPet || !foundPet) return null;

    return {
      matchId: value.matchId,
      foundPetId: value.foundPetId,
      matchedPetId: value.matchedPetId,
      score: value.score,
      status: value.status,
      matchedAt,
      scores: {
        visual: value.scores.visual,
        color: value.scores.color,
        spatial: value.scores.spatial,
        distanceMiles: value.scores.distanceMiles,
      },
      lostPet,
      foundPet,
    };
  }

  async function fetchMatches() {
    const loadRevision = ++matchLoadRevision;
    allMatches = [];
    try {
      const resp = await fetch('/api/v1/matches');
      if (!resp.ok) throw new Error(`Match API returned status ${resp.status}`);
      const payload = await resp.json();
      if (!Array.isArray(payload)) throw new Error('Match API returned a non-array payload');
      if (loadRevision !== matchLoadRevision) return;
      allMatches = payload.map(normalizeMatch).filter(match => match !== null);
      renderMatches();
    } catch (err) {
      if (loadRevision !== matchLoadRevision) return;
      console.error('Failed to fetch matches:', err);
      if (container) {
        container.replaceChildren(createElement('p', {
          text: 'Failed to load match records.',
          className: 'match-load-error',
        }));
      }
    }
  }

  function renderMatches() {
    if (!container) return;
    const minScore = parseFloat(scoreFilter?.value || '0.70');
    const filtered = allMatches.filter(m => m.score >= minScore);

    if (filtered.length === 0) {
      const emptyCard = createElement('div', {
        className: 'glass-card match-empty',
      });
      const emptyIcon = createElement('div', {
        text: '⌕',
        className: 'match-empty-icon',
      });
      emptyIcon.setAttribute('aria-hidden', 'true');
      emptyCard.append(
        emptyIcon,
        createElement('h3', {
          text: `No Candidate Matches Above ${Math.round(minScore * 100)}% Threshold`,
        }),
        createElement('p', {
          text: 'Try lowering the score filter threshold to see additional match candidates.',
          className: 'text-secondary',
        }),
      );
      container.replaceChildren(emptyCard);
      return;
    }

    container.replaceChildren(...filtered.map(createMatchCard));
    bindCardEvents();
  }

  function createImagePanel(label, pet, accentClass, includeName) {
    const panel = createElement('div', {
      className: 'image-panel',
    });
    const header = createElement('div', {
      className: 'image-panel-header',
    });
    header.append(createElement('span', {
      text: label,
      className: `image-panel-label ${accentClass}`,
    }));

    const zoomButton = createElement('button', {
      className: 'zoom-btn btn btn-secondary',
      text: pet.imageUrl ? '🔍 Zoom' : 'Image unavailable',
    });
    zoomButton.type = 'button';
    if (pet.imageUrl) {
      zoomButton.dataset.src = pet.imageUrl;
    } else {
      zoomButton.disabled = true;
    }
    header.append(zoomButton);
    panel.append(header);

    if (pet.imageUrl) {
      const image = createElement('img', {
        className: 'match-pet-image',
      });
      image.src = pet.imageUrl;
      image.alt = includeName ? `${pet.petName} photo` : 'Found pet photo';
      panel.append(image);
    } else {
      const placeholder = createElement('div', {
        text: 'Image unavailable',
        className: 'image-unavailable',
      });
      placeholder.setAttribute('role', 'img');
      placeholder.setAttribute('aria-label', includeName ? `${pet.petName} image unavailable` : 'Found pet image unavailable');
      panel.append(placeholder);
    }

    panel.append(
      createElement('h4', {
        text: includeName ? `${pet.petName} (${pet.breed})` : `Found Pet (${pet.breed})`,
        className: 'pet-name',
      }),
      createElement('p', {
        text: `${includeName ? 'Last Seen' : 'Found At'}: ${pet.location}`,
        className: 'pet-location',
      }),
    );
    return panel;
  }

  function createScore(scoreGrid, label, value, className) {
    const score = createElement('div');
    const row = createElement('div', {
      className: 'score-row',
    });
    row.append(
      createElement('span', { text: label }),
      createElement('span', { text: `${Math.round(value * 100)}%`, className: 'score-value' }),
    );
    const progress = createElement('progress', {
      className: `score-progress ${className}`,
    });
    progress.max = 100;
    progress.value = Math.round(value * 100);
    progress.setAttribute('aria-label', label);
    score.append(row, progress);
    scoreGrid.append(score);
  }

  function createActionButton(text, className, matchId, action) {
    const button = createElement('button', { className, text });
    button.type = 'button';
    button.dataset.matchId = matchId;
    if (action) button.dataset.action = action;
    return button;
  }

  function createMatchCard(m) {
    const scorePct = Math.round(m.score * 100);
    const badgeClass = scorePct >= 90 ? 'match-badge-high' : scorePct >= 80 ? 'match-badge-medium' : 'match-badge-low';
    const statusBadgeText = m.status === 'CONFIRMED' ? 'CONFIRMED REUNION' :
      m.status === 'REJECTED' ? 'REJECTED MATCH' :
        m.status === 'REUNITED' ? 'REUNITED' : `${scorePct}% HIGH CONFIDENCE MATCH`;

    const card = createElement('article', { className: 'glass-card match-card' });
    card.dataset.matchId = m.matchId;

    const summary = createElement('div', {
      className: 'match-summary',
    });
    const identity = createElement('div', {
      className: 'match-identity',
    });
    identity.append(
      createElement('span', {
        text: statusBadgeText,
        className: `match-badge ${badgeClass}`,
      }),
      createElement('span', {
        text: `Match ID: ${m.matchId}`,
        className: 'match-id',
      }),
    );
    summary.append(
      identity,
      createElement('span', {
        text: `Calculated: ${m.matchedAt.toLocaleString()}`,
        className: 'match-date',
      }),
    );

    const comparison = createElement('div', {
      className: 'match-comparison',
    });
    comparison.append(
      createImagePanel('Reported Lost Pet', m.lostPet, 'image-panel-label-lost', true),
      createImagePanel('Found Pet Candidate', m.foundPet, 'image-panel-label-found', false),
    );

    const scores = createElement('div', {
      className: 'match-scores',
    });
    scores.append(createElement('h4', {
      text: '✨ Gemma 4 AI Similarity Scoring Breakdown',
      className: 'match-scores-title',
    }));
    const scoreGrid = createElement('div', {
      className: 'score-grid',
    });
    createScore(scoreGrid, 'Visual Feature Match:', m.scores.visual, 'score-visual');
    createScore(scoreGrid, 'Color Alignment:', m.scores.color, 'score-color');
    createScore(scoreGrid, `Geospatial Proximity (${m.scores.distanceMiles} mi):`, m.scores.spatial, 'score-spatial');
    scores.append(scoreGrid);

    card.append(summary, comparison, scores);
    if (!identityEnabled) {
      const controls = createElement('div', {
        className: 'match-controls',
      });
      controls.append(
        createActionButton('💬 Contact Finder / Owner', 'btn btn-secondary contact-btn', m.matchId),
        createActionButton('Reject Match', 'btn btn-secondary action-btn', m.matchId, 'reject'),
        createActionButton('Confirm Reunion Match', 'btn btn-primary action-btn', m.matchId, 'confirm'),
      );
      const reunionButton = createActionButton('🎉 Mark as Reunited', 'btn btn-primary reunion-btn', m.matchId);
      reunionButton.dataset.petId = m.lostPet.petId;
      controls.append(reunionButton);
      card.append(controls);
    }
    return card;
  }

  function bindCardEvents() {
    // Zoom Handler
    container.querySelectorAll('.zoom-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const src = e.currentTarget.getAttribute('data-src');
        if (zoomedImage && zoomModal && src) {
          zoomedImage.src = src;
          openModal(zoomModal);
        }
      });
    });

    // Contact Handler
    container.querySelectorAll('.contact-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const matchId = e.currentTarget.getAttribute('data-match-id');
        const contactMatchIdInput = document.getElementById('contact-match-id');
        const contactModal = document.getElementById('contact-modal');
        if (contactMatchIdInput && contactModal) {
          contactMatchIdInput.value = matchId;
          openModal(contactModal);
        }
      });
    });

    // Reunion Resolution Handler
    container.querySelectorAll('.reunion-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const matchId = e.currentTarget.getAttribute('data-match-id');
        const petId = e.currentTarget.getAttribute('data-pet-id');
        const reunionMatchIdInput = document.getElementById('reunion-match-id');
        const reunionPetIdInput = document.getElementById('reunion-pet-id');
        const reunionModal = document.getElementById('reunion-modal');
        if (reunionModal && reunionMatchIdInput && reunionPetIdInput) {
          reunionMatchIdInput.value = matchId;
          reunionPetIdInput.value = petId;
          openModal(reunionModal);
        }
      });
    });

    // Action Handlers
    container.querySelectorAll('.action-btn').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        const matchId = e.currentTarget.getAttribute('data-match-id');
        const action = e.currentTarget.getAttribute('data-action');

        try {
          const resp = await fetch('/api/v1/matches/action', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ matchId: matchId, action: action })
          });

          if (resp.ok) {
            const res = await resp.json();
            showActionModal(res.status, action);
            fetchMatches();
          }
        } catch (err) {
          console.error('Action error:', err);
        }
      });
    });
  }

  // Bind Contact Form Submission
  const contactForm = document.getElementById('contact-form');
  if (contactForm) {
    contactForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const matchId = document.getElementById('contact-match-id')?.value || '';
      const senderEmail = document.getElementById('contact-sender-email')?.value || '';
      const message = document.getElementById('contact-message')?.value || '';

      try {
        const resp = await fetch('/api/v1/reunions/contact', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ matchId, senderEmail, message })
        });

        if (resp.ok) {
          closeModal(document.getElementById('contact-modal'));
          showActionModal('CONFIRMED', 'contact');
        }
      } catch (err) {
        console.error('Contact submit error:', err);
      }
    });
  }

  // Bind Reunion Resolution Form Submission
  const reunionForm = document.getElementById('reunion-form');
  if (reunionForm) {
    reunionForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const matchId = document.getElementById('reunion-match-id')?.value || '';
      const petId = document.getElementById('reunion-pet-id')?.value || '';
      const rating = parseInt(document.getElementById('reunion-rating')?.value || '5', 10);
      const feedback = document.getElementById('reunion-feedback')?.value || '';

      try {
        const resp = await fetch('/api/v1/reunions/resolve', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ matchId, petId, rating, feedback })
        });

        if (resp.ok) {
          closeModal(document.getElementById('reunion-modal'));
          showActionModal('REUNITED', 'resolve');
          fetchMatches();
        }
      } catch (err) {
        console.error('Reunion resolve error:', err);
      }
    });
  }

  function showActionModal(status, action) {
    const modal = document.getElementById('match-action-modal');
    const title = document.getElementById('action-modal-title');
    const desc = document.getElementById('action-modal-desc');

    if (title && desc && modal) {
      if (action === 'confirm') {
        title.textContent = 'Match Confirmed!';
        desc.textContent = 'Reunion status updated. Owner and finder notification alert dispatched.';
      } else {
        title.textContent = 'Match Rejected';
        desc.textContent = 'Match candidate removed from active list and feedback logged.';
      }
      openModal(modal);
    }
  }

  if (scoreFilter) {
    scoreFilter.addEventListener('change', () => renderMatches());
  }

  if (zoomModal) {
    zoomModal.addEventListener('click', () => closeModal(zoomModal));
  }

  document.querySelectorAll('.modal-close').forEach(button => {
    button.addEventListener('click', (event) => {
      closeModal(event.currentTarget.closest('.modal-overlay'));
    });
  });

  async function applyIdentityState(stateSnapshot) {
    identityEnabled = Boolean(stateSnapshot.enabled);
    if (!identityEnabled) {
      await fetchMatches();
      return;
    }
    if (stateSnapshot.unavailable || stateSnapshot.busy || !stateSnapshot.principal) {
      replaceMatchState('', 'match-auth-required');
      return;
    }
    await fetchMatches();
  }

  function scheduleIdentityState(stateSnapshot) {
    const principalKey = stateSnapshot.principal
      ? `${stateSnapshot.principal.issuer}\u0000${stateSnapshot.principal.subject}`
      : '';
    const stateKey = [
      stateSnapshot.enabled,
      stateSnapshot.unavailable,
      stateSnapshot.busy,
      principalKey,
      stateSnapshot.csrfToken,
    ].join('|');
    if (stateKey === lastIdentityState) return;
    lastIdentityState = stateKey;
    void applyIdentityState(stateSnapshot);
  }

  if (window.petspotrIdentity) {
    document.addEventListener('petspotr:identity-changed', (event) => {
      scheduleIdentityState(event.detail);
    });
    window.petspotrIdentity.ready.then(scheduleIdentityState);
  } else {
    void fetchMatches();
  }
});
