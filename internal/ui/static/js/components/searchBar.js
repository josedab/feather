/**
 * Search Bar Component
 */

function initSearchBar() {
    const searchInput = document.getElementById('global-search');
    if (!searchInput) return;

    // Debounce search
    let debounceTimer;
    searchInput.addEventListener('input', (e) => {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(() => {
            performSearch(e.target.value);
        }, 200);
    });

    // Keyboard shortcuts
    document.addEventListener('keydown', (e) => {
        // Cmd/Ctrl + K to focus search
        if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
            e.preventDefault();
            searchInput.focus();
        }
        // Escape to clear and blur
        if (e.key === 'Escape' && document.activeElement === searchInput) {
            searchInput.value = '';
            searchInput.blur();
            performSearch('');
        }
    });
}

function performSearch(query) {
    state.searchQuery = query.toLowerCase().trim();
    applyFilters();

    if (state.currentView === 'features') {
        renderFeatureList();
    }

    // Update URL with search query (optional)
    if (query) {
        history.replaceState(null, '', `#/?q=${encodeURIComponent(query)}`);
    } else if (window.location.hash.includes('?q=')) {
        history.replaceState(null, '', '#/');
    }
}

// Advanced search with operators
function parseSearchQuery(query) {
    const result = {
        text: '',
        filters: {},
    };

    // Parse special operators like "type:user" or "tag:ml"
    const operators = query.match(/(\w+):(\w+)/g) || [];
    operators.forEach(op => {
        const [key, value] = op.split(':');
        switch (key) {
            case 'type':
            case 'entity':
                result.filters.entityType = value;
                break;
            case 'tag':
                result.filters.tags = result.filters.tags || [];
                result.filters.tags.push(value);
                break;
            case 'category':
                result.filters.category = value;
                break;
            case 'status':
                result.filters.status = value;
                break;
            case 'owner':
                result.filters.owner = value;
                break;
        }
    });

    // Remove operators from text search
    result.text = query.replace(/(\w+):(\w+)/g, '').trim();

    return result;
}

// Search suggestions/autocomplete
function getSearchSuggestions(query) {
    if (!query || query.length < 2) return [];

    const suggestions = [];
    const lowerQuery = query.toLowerCase();

    // Feature name suggestions
    state.features.forEach(f => {
        if (f.name.toLowerCase().includes(lowerQuery)) {
            suggestions.push({
                type: 'feature',
                text: f.name,
                icon: 'cube',
            });
        }
    });

    // Entity type suggestions
    state.entityTypes.forEach(type => {
        if (type.toLowerCase().includes(lowerQuery)) {
            suggestions.push({
                type: 'entity',
                text: `type:${type}`,
                icon: 'users',
            });
        }
    });

    // Tag suggestions
    state.tags.forEach(tag => {
        if (tag.toLowerCase().includes(lowerQuery)) {
            suggestions.push({
                type: 'tag',
                text: `tag:${tag}`,
                icon: 'tag',
            });
        }
    });

    return suggestions.slice(0, 10);
}

// Initialize search on load
document.addEventListener('DOMContentLoaded', initSearchBar);
