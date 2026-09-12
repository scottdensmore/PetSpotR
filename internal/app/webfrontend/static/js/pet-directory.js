// Client-side controller for Public Pet Directory
document.addEventListener('DOMContentLoaded', () => {
  const filterForm = document.getElementById('pet-filter-form');
  const speciesSelect = document.getElementById('filter-species');
  const statusSelect = document.getElementById('filter-status');
  const queryInput = document.getElementById('filter-query');

  // Auto-submit or handle clean query parameters on change
  if (speciesSelect) {
    speciesSelect.addEventListener('change', () => {
      filterForm?.submit();
    });
  }

  if (statusSelect) {
    statusSelect.addEventListener('change', () => {
      filterForm?.submit();
    });
  }

  // Preserve keyboard navigation accessibility
  document.querySelectorAll('.pet-card').forEach((card) => {
    card.setAttribute('tabindex', '0');
  });
});
