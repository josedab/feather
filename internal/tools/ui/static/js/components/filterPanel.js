/**
 * Filter Panel Component
 */

function renderFilterPanel() {
    const panel = document.getElementById('filter-panel');
    if (!panel) return;

    const entityTypes = Array.from(state.entityTypes);
    const categories = Array.from(state.categories);
    const tags = Array.from(state.tags).slice(0, 10);

    const html = `
        <!-- Entity Type Filter -->
        <div class="mb-4">
            <label class="block text-sm font-medium text-gray-700 mb-1">Entity Type</label>
            <select id="filter-entity-type"
                    onchange="updateFilter('entityType', this.value)"
                    class="w-full px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-indigo-500">
                <option value="">All Types</option>
                ${entityTypes.map(type => `
                    <option value="${escapeHtml(type)}" ${state.filters.entityType === type ? 'selected' : ''}>
                        ${escapeHtml(type)}
                    </option>
                `).join('')}
            </select>
        </div>

        <!-- Category Filter -->
        ${categories.length > 0 ? `
            <div class="mb-4">
                <label class="block text-sm font-medium text-gray-700 mb-1">Category</label>
                <select id="filter-category"
                        onchange="updateFilter('category', this.value)"
                        class="w-full px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-indigo-500">
                    <option value="">All Categories</option>
                    ${categories.map(cat => `
                        <option value="${escapeHtml(cat)}" ${state.filters.category === cat ? 'selected' : ''}>
                            ${escapeHtml(cat)}
                        </option>
                    `).join('')}
                </select>
            </div>
        ` : ''}

        <!-- Status Filter -->
        <div class="mb-4">
            <label class="block text-sm font-medium text-gray-700 mb-1">Status</label>
            <select id="filter-status"
                    onchange="updateFilter('status', this.value)"
                    class="w-full px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-indigo-500">
                <option value="">All Statuses</option>
                <option value="active" ${state.filters.status === 'active' ? 'selected' : ''}>Active</option>
                <option value="experimental" ${state.filters.status === 'experimental' ? 'selected' : ''}>Experimental</option>
                <option value="deprecated" ${state.filters.status === 'deprecated' ? 'selected' : ''}>Deprecated</option>
                <option value="disabled" ${state.filters.status === 'disabled' ? 'selected' : ''}>Disabled</option>
            </select>
        </div>

        <!-- Tags Filter -->
        ${tags.length > 0 ? `
            <div class="mb-4">
                <label class="block text-sm font-medium text-gray-700 mb-2">Tags</label>
                <div class="space-y-2">
                    ${tags.map(tag => `
                        <label class="flex items-center">
                            <input type="checkbox"
                                   value="${escapeHtml(tag)}"
                                   ${state.filters.tags.includes(tag) ? 'checked' : ''}
                                   onchange="toggleTagFilter('${escapeHtml(tag)}')"
                                   class="h-4 w-4 text-indigo-600 border-gray-300 rounded focus:ring-indigo-500">
                            <span class="ml-2 text-sm text-gray-600">${escapeHtml(tag)}</span>
                        </label>
                    `).join('')}
                </div>
            </div>
        ` : ''}

        <!-- Clear Filters -->
        ${hasActiveFilters() ? `
            <button onclick="clearFilters()"
                    class="w-full px-3 py-2 text-sm text-red-600 border border-red-300 rounded-md hover:bg-red-50">
                Clear All Filters
            </button>
        ` : ''}
    `;

    panel.innerHTML = html;
}

function updateFilter(filterType, value) {
    if (value === '' || value === null) {
        state.filters[filterType] = null;
    } else {
        state.filters[filterType] = value;
    }
    applyFilters();
    renderFeatureList();
}

function toggleTagFilter(tag) {
    const index = state.filters.tags.indexOf(tag);
    if (index === -1) {
        state.filters.tags.push(tag);
    } else {
        state.filters.tags.splice(index, 1);
    }
    applyFilters();
    renderFeatureList();
}

function clearFilters() {
    state.filters = {
        entityType: null,
        category: null,
        tags: [],
        status: null,
    };
    state.searchQuery = '';

    // Clear search input
    const searchInput = document.getElementById('global-search');
    if (searchInput) {
        searchInput.value = '';
    }

    applyFilters();
    renderFilterPanel();
    renderFeatureList();
}

function hasActiveFilters() {
    return state.filters.entityType ||
           state.filters.category ||
           state.filters.status ||
           state.filters.tags.length > 0 ||
           state.searchQuery;
}

// Quick filter buttons
function addQuickFilters() {
    const quickFilters = [
        { label: 'Active', filter: { status: 'active' } },
        { label: 'ML Features', filter: { tags: ['ml'] } },
        { label: 'Real-time', filter: { tags: ['real-time'] } },
    ];

    return quickFilters.map(qf => `
        <button onclick='applyQuickFilter(${JSON.stringify(qf.filter)})'
                class="px-3 py-1 text-xs bg-gray-100 text-gray-700 rounded-full hover:bg-gray-200">
            ${qf.label}
        </button>
    `).join('');
}

function applyQuickFilter(filter) {
    Object.assign(state.filters, filter);
    applyFilters();
    renderFilterPanel();
    renderFeatureList();
}
