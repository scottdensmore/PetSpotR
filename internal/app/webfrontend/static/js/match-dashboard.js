// Client-side controller for Pet Match Comparison Dashboard
document.addEventListener('DOMContentLoaded', () => {
  const container = document.getElementById('matches-list-container');
  const scoreFilter = document.getElementById('scoreFilter');
  const zoomModal = document.getElementById('zoom-modal');
  const zoomedImage = document.getElementById('zoomed-image');
  const decisionStatus = document.getElementById('match-decision-status');
  const threadModal = document.getElementById('match-thread-modal');
  const threadMessages = document.getElementById('match-thread-messages');
  const threadEmpty = document.getElementById('match-thread-empty');
  const threadReadOnly = document.getElementById('match-thread-readonly');
  const threadForm = document.getElementById('match-thread-form');
  const threadMessage = document.getElementById('match-thread-message');
  const threadSend = document.getElementById('match-thread-send');
  const threadStatus = document.getElementById('match-thread-status');
  const threadError = document.getElementById('match-thread-error');

  let allMatches = [];
  let matchResultsLoaded = false;
  let identityEnabled = false;
  let matchLoadRevision = 0;
  let lastIdentityState = '';
  let decisionReturnTarget = null;
  let decisionInFlight = false;
  let identityRevision = 0;
  let identityPrincipalKey = '';
  let activeThread = null;
  let threadRevision = 0;
  let threadSendInFlight = false;
  let activeThreadSendToken = null;
  let threadReturnTarget = null;
  let pendingThreadAttempt = null;
  let threadIdentityFocusPending = false;
  const matchStatuses = new Set(['PENDING_REVIEW', 'CONFIRMED', 'REJECTED', 'REUNITED']);
  const allowedImageHosts = new Set(['storage.petspotr.io']);

  function openModal(modal) {
    if (modal) modal.hidden = false;
  }

  function closeModal(modal) {
    if (modal) modal.hidden = true;
  }

  function setDecisionBusy(button, busy) {
    container?.querySelectorAll('.action-btn').forEach(actionButton => {
      actionButton.setAttribute('aria-disabled', String(busy));
    });
    button.textContent = busy ? 'Saving decision...' : button.dataset.idleText;
    if (decisionStatus) {
      decisionStatus.textContent = busy ? 'Saving match decision...' : '';
      decisionStatus.hidden = !busy;
    }
  }

  function findDecisionReturnButton() {
    if (!decisionReturnTarget || !container) return null;
    return Array.from(container.querySelectorAll('.action-btn')).find(button =>
      button.dataset.matchId === decisionReturnTarget.matchId &&
      button.dataset.action === decisionReturnTarget.action,
    ) || null;
  }

  function closeActionModal() {
    const modal = document.getElementById('match-action-modal');
    closeModal(modal);
    const returnButton = findDecisionReturnButton();
    decisionReturnTarget = null;
    (returnButton || scoreFilter)?.focus();
  }

  function clearThreadStatus() {
    if (threadStatus) {
      threadStatus.textContent = '';
      threadStatus.hidden = true;
    }
    if (threadError) {
      threadError.textContent = '';
      threadError.hidden = true;
    }
  }

  function setThreadStatus(message, isError = false) {
    clearThreadStatus();
    const target = isError ? threadError : threadStatus;
    if (!target) return;
    target.textContent = message;
    target.hidden = false;
  }

  function setThreadSendBusy(busy) {
    threadSendInFlight = busy;
    if (threadForm) threadForm.setAttribute('aria-busy', String(busy));
    if (threadSend) {
      threadSend.setAttribute('aria-disabled', String(busy));
      threadSend.textContent = busy ? 'Sending message...' : 'Send message';
    }
    if (threadMessage) threadMessage.readOnly = busy;
    if (busy) setThreadStatus('Sending private message...');
  }

  function findThreadReturnButton() {
    if (!threadReturnTarget || !container) return null;
    return container.querySelector(`.message-btn[data-match-id="${CSS.escape(threadReturnTarget)}"]`);
  }

  function resetThreadView() {
    threadMessages?.replaceChildren();
    if (threadEmpty) threadEmpty.hidden = true;
    if (threadReadOnly) threadReadOnly.hidden = true;
    if (threadForm) threadForm.hidden = false;
    if (threadMessage) threadMessage.value = '';
    activeThreadSendToken = null;
    setThreadSendBusy(false);
    clearThreadStatus();
    pendingThreadAttempt = null;
  }

  function closeThreadModal(restoreFocus = true) {
    threadRevision += 1;
    const returnButton = restoreFocus ? findThreadReturnButton() : null;
    activeThread = null;
    threadReturnTarget = null;
    resetThreadView();
    closeModal(threadModal);
    if (restoreFocus) (returnButton || scoreFilter)?.focus();
  }

  function validMediatedMessageBody(value) {
    if (typeof value !== 'string' || value.length === 0 || Array.from(value).length > 1000) return false;
    return !Array.from(value).some(char => {
      const code = char.codePointAt(0);
      return (code < 0x20 && char !== '\n' && char !== '\r' && char !== '\t') || code === 0x7f;
    });
  }

  function normalizeThreadMessage(value) {
    if (!value || typeof value !== 'object' || !validRecordId(value.messageId)) return null;
    if (value.senderRole !== 'reporter' && value.senderRole !== 'finder') return null;
    if (!validMediatedMessageBody(value.message)) return null;
    if (typeof value.sentAt !== 'string' || value.sentAt.length > 64) return null;
    const sentAt = new Date(value.sentAt);
    if (Number.isNaN(sentAt.getTime())) return null;
    return {
      senderRole: value.senderRole,
      message: value.message,
      sentAt,
    };
  }

  function renderThreadMessages(messages) {
    if (!threadMessages) return;
    const normalized = Array.isArray(messages)
      ? messages.map(normalizeThreadMessage).filter(message => message !== null).slice(0, 100)
      : [];
    const items = normalized.map(message => {
      const item = createElement('li', { className: 'match-thread-message' });
      const meta = createElement('div', { className: 'match-thread-message-meta' });
      meta.append(
        createElement('strong', { text: message.senderRole === 'reporter' ? 'Reporter' : 'Finder' }),
        createElement('time', { text: message.sentAt.toLocaleString() }),
      );
      const body = createElement('p', { text: message.message, className: 'match-thread-message-body' });
      item.append(meta, body);
      return item;
    });
    threadMessages.replaceChildren(...items);
    if (threadEmpty) threadEmpty.hidden = items.length !== 0;
  }

  function threadIsWritable(status) {
    return status === 'PENDING_REVIEW' || status === 'CONFIRMED';
  }

  function applyThreadWritableState(status) {
    const writable = threadIsWritable(status);
    if (threadForm) threadForm.hidden = !writable;
    if (threadReadOnly) threadReadOnly.hidden = writable;
  }

  function newThreadIdempotencyKey() {
    if (typeof crypto.randomUUID === 'function') return `thread-${crypto.randomUUID()}`;
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    return `thread-${Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')}`;
  }

  function createElement(tagName, options = {}) {
    const element = document.createElement(tagName);
    if (options.className) element.className = options.className;
    if (options.text !== undefined) element.textContent = options.text;
    return element;
  }

  function renderMatchState(message, className, role) {
    allMatches = [];
    matchResultsLoaded = false;
    if (!container) return;
    if (!message) {
      container.replaceChildren();
      return;
    }
    const stateMessage = createElement('p', { text: message, className });
    stateMessage.setAttribute('role', role);
    if (role === 'status') stateMessage.setAttribute('aria-live', 'polite');
    container.replaceChildren(stateMessage);
  }

  function replaceMatchState(message, className) {
    matchLoadRevision += 1;
    renderMatchState(message, className, 'status');
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
    renderMatchState('Loading match records...', 'text-secondary', 'status');
    try {
      const resp = await fetch('/api/v1/matches');
      if (!resp.ok) throw new Error(`Match API returned status ${resp.status}`);
      const payload = await resp.json();
      if (!Array.isArray(payload)) throw new Error('Match API returned a non-array payload');
      if (loadRevision !== matchLoadRevision) return;
      allMatches = payload.map(normalizeMatch).filter(match => match !== null);
      matchResultsLoaded = true;
      renderMatches();
    } catch (err) {
      if (loadRevision !== matchLoadRevision) return;
      console.error('Failed to fetch matches:', err);
      renderMatchState('Failed to load match records.', 'match-load-error', 'alert');
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
    const controls = createElement('div', {
      className: 'match-controls',
    });
    if (identityEnabled) {
      const messageButton = createActionButton(
        'Open private messages', 'btn btn-secondary message-btn', m.matchId,
      );
      messageButton.dataset.matchStatus = m.status;
      controls.append(
        messageButton,
        createActionButton('Reject Match', 'btn btn-secondary action-btn', m.matchId, 'reject'),
        createActionButton('Confirm Match', 'btn btn-primary action-btn', m.matchId, 'confirm'),
      );
    } else {
      controls.append(
        createActionButton('💬 Contact Finder / Owner', 'btn btn-secondary contact-btn', m.matchId),
        createActionButton('Reject Match', 'btn btn-secondary action-btn', m.matchId, 'reject'),
        createActionButton('Confirm Reunion Match', 'btn btn-primary action-btn', m.matchId, 'confirm'),
      );
      const reunionButton = createActionButton('🎉 Mark as Reunited', 'btn btn-primary reunion-btn', m.matchId);
      reunionButton.dataset.petId = m.lostPet.petId;
      controls.append(reunionButton);
    }
    card.append(controls);
    return card;
  }

  async function loadThread(match, loadRevision, loadIdentityRevision) {
    setThreadStatus('Loading private messages...');
    try {
      const response = await fetch(`/api/v1/reunions/contact?matchId=${encodeURIComponent(match.matchId)}`);
      if (loadRevision !== threadRevision || loadIdentityRevision !== identityRevision ||
          activeThread?.matchId !== match.matchId) return false;
      if (!response.ok) throw new Error(`Private thread API returned status ${response.status}`);
      const payload = await response.json();
      if (loadRevision !== threadRevision || loadIdentityRevision !== identityRevision ||
          activeThread?.matchId !== match.matchId) return false;
      if (!payload || payload.matchId !== match.matchId || !Array.isArray(payload.messages)) {
        throw new Error('Private thread API returned an invalid payload');
      }
      renderThreadMessages(payload.messages);
      applyThreadWritableState(match.status);
      clearThreadStatus();
      return true;
    } catch (error) {
      if (loadRevision !== threadRevision || loadIdentityRevision !== identityRevision ||
          activeThread?.matchId !== match.matchId) return false;
      console.error('Failed to load private match messages:', error);
      threadMessages?.replaceChildren();
      if (threadEmpty) threadEmpty.hidden = true;
      setThreadStatus('Private messages could not be loaded. Try again.', true);
      return false;
    }
  }

  function openThreadModal(matchID, status) {
    const match = allMatches.find(candidate => candidate.matchId === matchID);
    if (!match || match.status !== status || !threadModal) return;
    threadRevision += 1;
    const loadRevision = threadRevision;
    const loadIdentityRevision = identityRevision;
    activeThread = { matchId: matchID, status };
    threadReturnTarget = matchID;
    resetThreadView();
    applyThreadWritableState(status);
    const matchIDInput = document.getElementById('match-thread-match-id');
    if (matchIDInput) matchIDInput.value = matchID;
    openModal(threadModal);
    threadModal.querySelector('.match-thread-close')?.focus();
    void loadThread(activeThread, loadRevision, loadIdentityRevision);
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

    // Authenticated participant message handler
    container.querySelectorAll('.message-btn').forEach(btn => {
      btn.addEventListener('click', (event) => {
        const button = event.currentTarget;
        openThreadModal(button.dataset.matchId || '', button.dataset.matchStatus || '');
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
        const button = e.currentTarget;
        if (decisionInFlight || button.getAttribute('aria-disabled') === 'true') return;
        const matchId = button.getAttribute('data-match-id');
        const action = button.getAttribute('data-action');
        decisionInFlight = true;
        const decisionIdentityRevision = identityRevision;
        const decisionUsesIdentity = identityEnabled;
        button.dataset.idleText = button.textContent;
        setDecisionBusy(button, true);

        try {
          let identityState = null;
          if (identityEnabled && window.petspotrIdentity) {
            identityState = await window.petspotrIdentity.requireSession();
          }
          if (decisionUsesIdentity && decisionIdentityRevision !== identityRevision) return;

          const headers = { 'Content-Type': 'application/json' };
          if (identityState?.enabled) headers['X-CSRF-Token'] = identityState.csrfToken;
          const resp = await fetch('/api/v1/matches/action', {
            method: 'POST',
            headers,
            body: JSON.stringify({ matchId: matchId, action: action })
          });
          if (decisionUsesIdentity && decisionIdentityRevision !== identityRevision) return;

          if (!resp.ok) {
            const message = resp.status === 409
              ? 'A different decision was already recorded for this participant.'
              : 'Your decision could not be saved. Please try again.';
            showActionModal('', 'decision-error', message, { matchId, action });
            return;
          }
          const res = await resp.json();
          await fetchMatches();
          if (decisionUsesIdentity && decisionIdentityRevision !== identityRevision) return;
          showActionModal(res.status, action, '', { matchId, action });
        } catch (err) {
          console.error('Action error:', err);
          if (decisionUsesIdentity && decisionIdentityRevision !== identityRevision) return;
          if (err.code === 'identity-required') window.petspotrIdentity?.focusSignIn();
          const message = err.code === 'identity-required'
            ? 'Sign in again before recording a match decision.'
            : 'Your decision could not be saved. Please try again.';
          showActionModal('', 'decision-error', message, { matchId, action });
        } finally {
          decisionInFlight = false;
          setDecisionBusy(button, false);
        }
      });
    });
  }

  if (threadForm) {
    threadForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (threadSendInFlight || !activeThread || !threadIsWritable(activeThread.status)) return;
      const message = threadMessage?.value.trim() || '';
      if (!validMediatedMessageBody(message)) {
        setThreadStatus('Enter a message of up to 1,000 characters.', true);
        threadMessage?.focus();
        return;
      }

      const attempt = pendingThreadAttempt?.matchId === activeThread.matchId &&
        pendingThreadAttempt.message === message
        ? pendingThreadAttempt
        : { matchId: activeThread.matchId, message, key: newThreadIdempotencyKey() };
      pendingThreadAttempt = attempt;
      const sendRevision = threadRevision;
      const sendIdentityRevision = identityRevision;
      const sendToken = Symbol('private-thread-send');
      activeThreadSendToken = sendToken;
      setThreadSendBusy(true);

      try {
        const identityState = await window.petspotrIdentity?.requireSession();
        if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
        if (!identityState?.enabled) throw new Error('identity-required');
        const response = await fetch('/api/v1/reunions/contact', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': identityState.csrfToken,
            'Idempotency-Key': attempt.key,
          },
          body: JSON.stringify({ matchId: attempt.matchId, message: attempt.message }),
        });
        if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
        if (!response.ok) {
          const error = new Error(`Private message API returned status ${response.status}`);
          error.status = response.status;
          throw error;
        }
        pendingThreadAttempt = null;
        if (threadMessage) threadMessage.value = '';
        const refreshed = await loadThread(activeThread, sendRevision, sendIdentityRevision);
        if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
        if (!refreshed) {
          setThreadStatus(
            'Private message was sent, but the conversation could not be refreshed. Close and reopen private messages.',
            true,
          );
          threadMessage?.focus();
          return;
        }
        setThreadStatus('Private message sent.');
        threadMessage?.focus();
      } catch (error) {
        if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
        console.error('Private message submit error:', error);
        const message = error.status === 409
          ? 'This message conflicts with the conversation or the conversation is read-only.'
          : 'Private message not sent. Try again.';
        setThreadStatus(message, true);
        threadMessage?.focus();
      } finally {
        if (activeThreadSendToken === sendToken) {
          activeThreadSendToken = null;
          if (sendRevision === threadRevision) setThreadSendBusy(false);
        }
      }
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

  function showActionModal(status, action, errorMessage = '', returnTarget = null) {
    const modal = document.getElementById('match-action-modal');
    const icon = document.getElementById('action-modal-icon');
    const title = document.getElementById('action-modal-title');
    const desc = document.getElementById('action-modal-desc');

    if (title && desc && modal) {
      decisionReturnTarget = returnTarget;
      if (action === 'decision-error') {
        if (icon) icon.hidden = true;
        title.textContent = 'Decision not saved';
        desc.textContent = errorMessage;
      } else if (action === 'confirm' && status === 'PENDING_REVIEW') {
        if (icon) icon.hidden = false;
        title.textContent = 'Decision recorded';
        desc.textContent = 'Waiting for the other participant to confirm this match.';
      } else if (action === 'confirm') {
        if (icon) icon.hidden = false;
        title.textContent = 'Match confirmed';
        desc.textContent = 'Both participants confirmed this match.';
      } else {
        if (icon) icon.hidden = false;
        title.textContent = 'Match Rejected';
        desc.textContent = 'Match candidate removed from active list and feedback logged.';
      }
      openModal(modal);
      modal.querySelector('.modal-close')?.focus();
    }
  }

  if (scoreFilter) {
    scoreFilter.addEventListener('change', () => {
      if (matchResultsLoaded) renderMatches();
    });
  }

  if (zoomModal) {
    zoomModal.addEventListener('click', () => closeModal(zoomModal));
  }

  document.querySelectorAll('.modal-close').forEach(button => {
    button.addEventListener('click', (event) => {
      const modal = event.currentTarget.closest('.modal-overlay');
      if (modal?.id === 'match-action-modal') {
        closeActionModal();
      } else {
        closeModal(modal);
      }
    });
  });

  document.querySelectorAll('.match-thread-close').forEach(button => {
    button.addEventListener('click', () => closeThreadModal());
  });

  threadModal?.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeThreadModal();
      return;
    }
    if (event.key !== 'Tab') return;
    const focusable = Array.from(threadModal.querySelectorAll(
      'button:not([disabled]):not([hidden]), textarea:not([disabled]):not([hidden]), input:not([disabled]):not([hidden])',
    )).filter(element => element.getClientRects().length > 0);
    if (focusable.length === 0) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  });

  const actionModal = document.getElementById('match-action-modal');
  actionModal?.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeActionModal();
      return;
    }
    if (event.key === 'Tab') {
      event.preventDefault();
      actionModal.querySelector('.modal-close')?.focus();
    }
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
    identityRevision += 1;
    if (activeThread && stateSnapshot.enabled && (
      stateSnapshot.unavailable || stateSnapshot.busy || !stateSnapshot.principal ||
      (identityPrincipalKey && identityPrincipalKey !== principalKey)
    )) {
      threadIdentityFocusPending = Boolean(threadModal?.contains(document.activeElement));
      closeThreadModal(false);
    }
    if (threadIdentityFocusPending && stateSnapshot.enabled && !stateSnapshot.busy) {
      threadIdentityFocusPending = false;
      queueMicrotask(() => {
        if (stateSnapshot.unavailable) {
          scoreFilter?.focus();
        } else if (stateSnapshot.principal) {
          document.getElementById('identity-sign-out')?.focus();
        } else {
          window.petspotrIdentity?.focusSignIn();
        }
      });
    }
    identityPrincipalKey = principalKey;
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
