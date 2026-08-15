// Client-side controller for Pet Match Comparison Dashboard
document.addEventListener('DOMContentLoaded', () => {
  const container = document.getElementById('matches-list-container');
  const scoreFilter = document.getElementById('scoreFilter');
  const zoomModal = document.getElementById('zoom-modal');
  const zoomedImage = document.getElementById('zoomed-image');

  let allMatches = [];
  const matchStatuses = new Set(['PENDING_REVIEW', 'CONFIRMED', 'REJECTED', 'REUNITED']);
  const allowedImageHosts = new Set(['storage.petspotr.io']);

  function createElement(tagName, options = {}) {
    const element = document.createElement(tagName);
    if (options.className) element.className = options.className;
    if (options.text !== undefined) element.textContent = options.text;
    if (options.style) element.style.cssText = options.style;
    return element;
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
    try {
      const resp = await fetch('/api/v1/matches');
      if (resp.ok) {
        const payload = await resp.json();
        if (!Array.isArray(payload)) throw new Error('Match API returned a non-array payload');
        allMatches = payload.map(normalizeMatch).filter(match => match !== null);
        renderMatches();
      }
    } catch (err) {
      console.error('Failed to fetch matches:', err);
      if (container) {
        container.replaceChildren(createElement('p', {
          text: 'Failed to load match records.',
          style: 'text-align: center; color: var(--status-lost);',
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
        className: 'glass-card',
        style: 'text-align: center; padding: 3rem;',
      });
      const emptyIcon = createElement('div', {
        text: '⌕',
        style: 'color: var(--text-muted); font-size: 3rem; line-height: 1; margin-bottom: 1rem;',
      });
      emptyIcon.setAttribute('aria-hidden', 'true');
      emptyCard.append(
        emptyIcon,
        createElement('h3', {
          text: `No Candidate Matches Above ${Math.round(minScore * 100)}% Threshold`,
          style: 'font-size: 1.25rem; margin-bottom: 0.5rem;',
        }),
        createElement('p', {
          text: 'Try lowering the score filter threshold to see additional match candidates.',
          style: 'color: var(--text-secondary);',
        }),
      );
      container.replaceChildren(emptyCard);
      return;
    }

    container.replaceChildren(...filtered.map(createMatchCard));
    bindCardEvents();
  }

  function createImagePanel(label, pet, accentColor, includeName) {
    const panel = createElement('div', {
      style: 'background: var(--bg-primary); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--border-color); text-align: center;',
    });
    const header = createElement('div', {
      style: 'display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem;',
    });
    header.append(createElement('span', {
      text: label,
      style: `font-size: 0.85rem; font-weight: 700; color: ${accentColor}; text-transform: uppercase;`,
    }));

    const zoomButton = createElement('button', {
      className: 'zoom-btn btn btn-secondary',
      text: pet.imageUrl ? '🔍 Zoom' : 'Image unavailable',
      style: 'min-width: 44px; min-height: 44px; padding: 0.25rem 0.5rem; font-size: 0.75rem;',
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
        style: 'width: 100%; height: 220px; object-fit: cover; border-radius: var(--radius-sm); margin-bottom: 0.75rem;',
      });
      image.src = pet.imageUrl;
      image.alt = includeName ? `${pet.petName} photo` : 'Found pet photo';
      panel.append(image);
    } else {
      const placeholder = createElement('div', {
        text: 'Image unavailable',
        style: 'width: 100%; height: 220px; display: grid; place-items: center; background: var(--bg-secondary); color: var(--text-secondary); border-radius: var(--radius-sm); margin-bottom: 0.75rem;',
      });
      placeholder.setAttribute('role', 'img');
      placeholder.setAttribute('aria-label', includeName ? `${pet.petName} image unavailable` : 'Found pet image unavailable');
      panel.append(placeholder);
    }

    panel.append(
      createElement('h4', {
        text: includeName ? `${pet.petName} (${pet.breed})` : `Found Pet (${pet.breed})`,
        style: 'font-size: 1.1rem;',
      }),
      createElement('p', {
        text: `${includeName ? 'Last Seen' : 'Found At'}: ${pet.location}`,
        style: 'font-size: 0.85rem; color: var(--text-secondary);',
      }),
    );
    return panel;
  }

  function createScore(scoreGrid, label, value, color) {
    const score = createElement('div');
    const row = createElement('div', {
      style: 'display: flex; justify-content: space-between; font-size: 0.85rem; margin-bottom: 0.35rem;',
    });
    row.append(
      createElement('span', { text: label }),
      createElement('span', { text: `${Math.round(value * 100)}%`, style: 'font-weight: 700;' }),
    );
    const track = createElement('div', {
      style: 'height: 8px; background: rgba(255,255,255,0.1); border-radius: 4px; overflow: hidden;',
    });
    const bar = createElement('div', {
      style: `height: 100%; background: ${color};`,
    });
    bar.style.width = `${Math.round(value * 100)}%`;
    track.append(bar);
    score.append(row, track);
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
    const badgeColor = scorePct >= 90 ? 'var(--status-reunited)' : scorePct >= 80 ? 'var(--brand-primary-hover)' : 'var(--status-matched)';
    const badgeTextColor = scorePct >= 90 ? '#052e16' : scorePct >= 80 ? '#ffffff' : '#451a03';
    const statusBadgeText = m.status === 'CONFIRMED' ? 'CONFIRMED REUNION' :
      m.status === 'REJECTED' ? 'REJECTED MATCH' :
        m.status === 'REUNITED' ? 'REUNITED' : `${scorePct}% HIGH CONFIDENCE MATCH`;

    const card = createElement('article', { className: 'glass-card' });
    card.dataset.matchId = m.matchId;
    card.style.overflowWrap = 'anywhere';

    const summary = createElement('div', {
      style: 'display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; flex-wrap: wrap; gap: 0.75rem;',
    });
    const identity = createElement('div', {
      style: 'display: flex; align-items: center; gap: 0.75rem;',
    });
    identity.append(
      createElement('span', {
        text: statusBadgeText,
        style: `background: ${badgeColor}; color: ${badgeTextColor}; padding: 0.35rem 0.85rem; border-radius: var(--radius-full); font-size: 0.85rem; font-weight: 700; text-transform: uppercase;`,
      }),
      createElement('span', {
        text: `Match ID: ${m.matchId}`,
        style: 'font-size: 0.9rem; color: var(--text-secondary);',
      }),
    );
    summary.append(
      identity,
      createElement('span', {
        text: `Calculated: ${m.matchedAt.toLocaleString()}`,
        style: 'font-size: 0.85rem; color: var(--text-muted);',
      }),
    );

    const comparison = createElement('div', {
      style: 'display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.5rem; margin-bottom: 1.75rem;',
    });
    comparison.append(
      createImagePanel('Reported Lost Pet', m.lostPet, 'var(--status-lost)', true),
      createImagePanel('Found Pet Candidate', m.foundPet, 'var(--status-found)', false),
    );

    const scores = createElement('div', {
      style: 'background: var(--bg-secondary); padding: 1.5rem; border-radius: var(--radius-md); border: 1px solid var(--border-color); margin-bottom: 1.75rem;',
    });
    scores.append(createElement('h4', {
      text: '✨ Gemma 4 AI Similarity Scoring Breakdown',
      style: 'font-size: 1rem; margin-bottom: 1rem; color: var(--brand-primary);',
    }));
    const scoreGrid = createElement('div', {
      style: 'display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1.25rem;',
    });
    createScore(scoreGrid, 'Visual Feature Match:', m.scores.visual, 'var(--brand-primary)');
    createScore(scoreGrid, 'Color Alignment:', m.scores.color, 'var(--brand-secondary)');
    createScore(scoreGrid, `Geospatial Proximity (${m.scores.distanceMiles} mi):`, m.scores.spatial, 'var(--status-reunited)');
    scores.append(scoreGrid);

    const controls = createElement('div', {
      style: 'display: flex; justify-content: flex-end; gap: 0.75rem; flex-wrap: wrap;',
    });
    controls.append(
      createActionButton('💬 Contact Finder / Owner', 'btn btn-secondary contact-btn', m.matchId),
      createActionButton('Reject Match', 'btn btn-secondary action-btn', m.matchId, 'reject'),
      createActionButton('Confirm Reunion Match', 'btn btn-primary action-btn', m.matchId, 'confirm'),
    );
    const reunionButton = createActionButton('🎉 Mark as Reunited', 'btn btn-primary reunion-btn', m.matchId);
    reunionButton.dataset.petId = m.lostPet.petId;
    reunionButton.style.background = 'var(--status-reunited)';
    controls.append(reunionButton);

    card.append(summary, comparison, scores, controls);
    return card;
  }

  function bindCardEvents() {
    // Zoom Handler
    container.querySelectorAll('.zoom-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const src = e.currentTarget.getAttribute('data-src');
        if (zoomedImage && zoomModal && src) {
          zoomedImage.src = src;
          zoomModal.style.display = 'flex';
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
          contactModal.style.display = 'flex';
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
          reunionModal.style.display = 'flex';
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
          document.getElementById('contact-modal').style.display = 'none';
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
          document.getElementById('reunion-modal').style.display = 'none';
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
      modal.style.display = 'flex';
    }
  }

  if (scoreFilter) {
    scoreFilter.addEventListener('change', () => renderMatches());
  }

  fetchMatches();
});
