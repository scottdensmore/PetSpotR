// Client-side controller for Pet Match Comparison Dashboard
document.addEventListener('DOMContentLoaded', () => {
  const container = document.getElementById('matches-list-container');
  const scoreFilter = document.getElementById('scoreFilter');
  const zoomModal = document.getElementById('zoom-modal');
  const zoomedImage = document.getElementById('zoomed-image');

  let allMatches = [];

  async function fetchMatches() {
    try {
      const resp = await fetch('/api/v1/matches');
      if (resp.ok) {
        allMatches = await resp.json();
        renderMatches();
      }
    } catch (err) {
      console.error('Failed to fetch matches:', err);
      if (container) {
        container.innerHTML = '<p style="text-align: center; color: var(--status-lost);">Failed to load match records.</p>';
      }
    }
  }

  function renderMatches() {
    if (!container) return;
    const minScore = parseFloat(scoreFilter?.value || '0.70');
    const filtered = allMatches.filter(m => m.score >= minScore);

    if (filtered.length === 0) {
      container.innerHTML = `
        <div class="glass-card" style="text-align: center; padding: 3rem;">
          <svg width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24" style="color: var(--text-muted); margin-bottom: 1rem;"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.172 9.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
          <h3 style="font-size: 1.25rem; margin-bottom: 0.5rem;">No Candidate Matches Above ${Math.round(minScore * 100)}% Threshold</h3>
          <p style="color: var(--text-secondary);">Try lowering the score filter threshold to see additional match candidates.</p>
        </div>
      `;
      return;
    }

    container.innerHTML = filtered.map(m => createMatchCardHTML(m)).join('');
    bindCardEvents();
  }

  function createMatchCardHTML(m) {
    const scorePct = Math.round(m.score * 100);
    const badgeColor = scorePct >= 90 ? 'var(--status-reunited)' : scorePct >= 80 ? 'var(--brand-primary)' : 'var(--status-matched)';
    const statusBadgeText = m.status === 'CONFIRMED' ? 'CONFIRMED REUNION' : m.status === 'REJECTED' ? 'REJECTED MATCH' : `${scorePct}% HIGH CONFIDENCE MATCH`;

    return `
      <article class="glass-card" data-match-id="${m.matchId}">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; flex-wrap: wrap; gap: 0.75rem;">
          <div style="display: flex; align-items: center; gap: 0.75rem;">
            <span style="background: ${badgeColor}; color: #ffffff; padding: 0.35rem 0.85rem; border-radius: var(--radius-full); font-size: 0.85rem; font-weight: 700; text-transform: uppercase;">
              ${statusBadgeText}
            </span>
            <span style="font-size: 0.9rem; color: var(--text-secondary);">Match ID: ${m.matchId}</span>
          </div>
          <span style="font-size: 0.85rem; color: var(--text-muted);">Calculated: ${new Date(m.matchedAt).toLocaleString()}</span>
        </div>

        <!-- Side-by-Side Visual Image Comparison -->
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.5rem; margin-bottom: 1.75rem;">
          <!-- Lost Pet Side -->
          <div style="background: var(--bg-primary); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--border-color); text-align: center;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem;">
              <span style="font-size: 0.85rem; font-weight: 700; color: var(--status-lost); text-transform: uppercase;">Reported Lost Pet</span>
              <button class="zoom-btn btn btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" data-src="${m.lostPet.imageUrl}">🔍 Zoom</button>
            </div>
            <img src="${m.lostPet.imageUrl}" alt="Lost Pet" style="width: 100%; height: 220px; object-fit: cover; border-radius: var(--radius-sm); margin-bottom: 0.75rem;">
            <h4 style="font-size: 1.1rem;">${m.lostPet.petName} (${m.lostPet.breed})</h4>
            <p style="font-size: 0.85rem; color: var(--text-secondary);">Last Seen: ${m.lostPet.location}</p>
          </div>

          <!-- Found Pet Side -->
          <div style="background: var(--bg-primary); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--border-color); text-align: center;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem;">
              <span style="font-size: 0.85rem; font-weight: 700; color: var(--status-found); text-transform: uppercase;">Found Pet Candidate</span>
              <button class="zoom-btn btn btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" data-src="${m.foundPet.imageUrl}">🔍 Zoom</button>
            </div>
            <img src="${m.foundPet.imageUrl}" alt="Found Pet" style="width: 100%; height: 220px; object-fit: cover; border-radius: var(--radius-sm); margin-bottom: 0.75rem;">
            <h4 style="font-size: 1.1rem;">Found Pet (${m.foundPet.breed})</h4>
            <p style="font-size: 0.85rem; color: var(--text-secondary);">Found At: ${m.foundPet.location}</p>
          </div>
        </div>

        <!-- Similarity Scoring Component Breakdown -->
        <div style="background: var(--bg-secondary); padding: 1.5rem; border-radius: var(--radius-md); border: 1px solid var(--border-color); margin-bottom: 1.75rem;">
          <h4 style="font-size: 1rem; margin-bottom: 1rem; color: var(--brand-primary);">✨ Gemma 4 AI Similarity Scoring Breakdown</h4>
          <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1.25rem;">
            <div>
              <div style="display: flex; justify-content: space-between; font-size: 0.85rem; margin-bottom: 0.35rem;">
                <span>Visual Feature Match:</span>
                <span style="font-weight: 700;">${Math.round(m.scores.visual * 100)}%</span>
              </div>
              <div style="height: 8px; background: rgba(255,255,255,0.1); border-radius: 4px; overflow: hidden;">
                <div style="width: ${Math.round(m.scores.visual * 100)}%; height: 100%; background: var(--brand-primary);"></div>
              </div>
            </div>

            <div>
              <div style="display: flex; justify-content: space-between; font-size: 0.85rem; margin-bottom: 0.35rem;">
                <span>Color Alignment:</span>
                <span style="font-weight: 700;">${Math.round(m.scores.color * 100)}%</span>
              </div>
              <div style="height: 8px; background: rgba(255,255,255,0.1); border-radius: 4px; overflow: hidden;">
                <div style="width: ${Math.round(m.scores.color * 100)}%; height: 100%; background: var(--brand-secondary);"></div>
              </div>
            </div>

            <div>
              <div style="display: flex; justify-content: space-between; font-size: 0.85rem; margin-bottom: 0.35rem;">
                <span>Geospatial Proximity (${m.scores.distanceMiles} mi):</span>
                <span style="font-weight: 700;">${Math.round(m.scores.spatial * 100)}%</span>
              </div>
              <div style="height: 8px; background: rgba(255,255,255,0.1); border-radius: 4px; overflow: hidden;">
                <div style="width: ${Math.round(m.scores.spatial * 100)}%; height: 100%; background: var(--status-reunited);"></div>
              </div>
            </div>
          </div>
        </div>

        <!-- Action Controls -->
        <div style="display: flex; justify-content: flex-end; gap: 1rem; flex-wrap: wrap;">
          <button class="btn btn-secondary action-btn" data-action="reject" data-match-id="${m.matchId}">Reject Match</button>
          <button class="btn btn-primary action-btn" data-action="confirm" data-match-id="${m.matchId}">Confirm Reunion Match</button>
        </div>
      </article>
    `;
  }

  function bindCardEvents() {
    // Zoom Handler
    document.querySelectorAll('.zoom-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const src = e.currentTarget.getAttribute('data-src');
        if (zoomedImage && zoomModal && src) {
          zoomedImage.src = src;
          zoomModal.style.display = 'flex';
        }
      });
    });

    // Action Handlers
    document.querySelectorAll('.action-btn').forEach(btn => {
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
