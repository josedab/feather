/**
 * Feature List Component
 */

function renderFeatureList() {
    const content = document.getElementById('content');
    const features = state.filteredFeatures;

    if (features.length === 0) {
        content.innerHTML = `
            <div class="text-center py-12">
                <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M12 12h.01M12 12h.01M12 12h.01M12 12h.01M12 12h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                </svg>
                <h3 class="mt-2 text-sm font-medium text-gray-900">No features found</h3>
                <p class="mt-1 text-sm text-gray-500">Try adjusting your search or filter criteria.</p>
            </div>
        `;
        return;
    }

    const html = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-900">Feature Catalog</h1>
            <p class="mt-1 text-sm text-gray-500">${features.length} feature${features.length !== 1 ? 's' : ''} found</p>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            ${features.map(f => renderFeatureCard(f)).join('')}
        </div>
    `;

    content.innerHTML = html;
}

function renderFeatureCard(feature) {
    const tags = (feature.tags || []).slice(0, 3);
    const dataTypeClass = formatDataType(feature.data_type);
    const statusClass = formatStatus(feature.status);

    return `
        <div class="feature-card bg-white rounded-lg shadow border border-gray-200 p-4 cursor-pointer transition-all"
             onclick="showFeatureDetail('${escapeHtml(feature.name)}')">
            <div class="flex items-start justify-between">
                <div class="flex-1 min-w-0">
                    <h3 class="text-sm font-semibold text-gray-900 truncate">${escapeHtml(feature.name)}</h3>
                    <p class="mt-1 text-xs text-gray-500 line-clamp-2">${escapeHtml(feature.description || 'No description')}</p>
                </div>
                <span class="ml-2 px-2 py-1 text-xs rounded ${dataTypeClass}">${feature.data_type}</span>
            </div>

            <div class="mt-3 flex items-center space-x-2">
                <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">
                    ${escapeHtml(feature.entity_type || 'unknown')}
                </span>
                <span class="px-2 py-0.5 rounded text-xs ${statusClass}">${feature.status || 'active'}</span>
            </div>

            ${tags.length > 0 ? `
                <div class="mt-3 flex flex-wrap gap-1">
                    ${tags.map(tag => `
                        <span class="px-2 py-0.5 text-xs bg-indigo-50 text-indigo-700 rounded">${escapeHtml(tag)}</span>
                    `).join('')}
                    ${feature.tags && feature.tags.length > 3 ? `
                        <span class="px-2 py-0.5 text-xs text-gray-500">+${feature.tags.length - 3} more</span>
                    ` : ''}
                </div>
            ` : ''}

            ${feature.owner ? `
                <div class="mt-3 flex items-center text-xs text-gray-500">
                    <svg class="mr-1 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
                    </svg>
                    ${escapeHtml(feature.owner)}
                </div>
            ` : ''}
        </div>
    `;
}

function renderEntityTypes() {
    const content = document.getElementById('content');
    const entityTypes = Array.from(state.entityTypes);

    // Group features by entity type
    const entityGroups = {};
    state.features.forEach(f => {
        const type = f.entity_type || 'unknown';
        if (!entityGroups[type]) {
            entityGroups[type] = [];
        }
        entityGroups[type].push(f);
    });

    const html = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-900">Entity Types</h1>
            <p class="mt-1 text-sm text-gray-500">${entityTypes.length} entity type${entityTypes.length !== 1 ? 's' : ''}</p>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            ${entityTypes.map(type => `
                <div class="bg-white rounded-lg shadow border border-gray-200 p-6">
                    <div class="flex items-center justify-between">
                        <h3 class="text-lg font-semibold text-gray-900">${escapeHtml(type)}</h3>
                        <span class="px-3 py-1 text-sm bg-indigo-100 text-indigo-700 rounded-full">
                            ${entityGroups[type].length} features
                        </span>
                    </div>

                    <div class="mt-4">
                        <h4 class="text-sm font-medium text-gray-700">Features:</h4>
                        <ul class="mt-2 space-y-1">
                            ${entityGroups[type].slice(0, 5).map(f => `
                                <li class="text-sm text-gray-600 truncate">
                                    <a href="#/feature/${encodeURIComponent(f.name)}" class="hover:text-indigo-600">
                                        ${escapeHtml(f.name)}
                                    </a>
                                </li>
                            `).join('')}
                            ${entityGroups[type].length > 5 ? `
                                <li class="text-sm text-gray-400">
                                    +${entityGroups[type].length - 5} more
                                </li>
                            ` : ''}
                        </ul>
                    </div>

                    <button onclick="filterByEntityType('${escapeHtml(type)}')"
                            class="mt-4 w-full px-4 py-2 text-sm text-indigo-600 border border-indigo-600 rounded hover:bg-indigo-50">
                        View All Features
                    </button>
                </div>
            `).join('')}
        </div>
    `;

    content.innerHTML = html;
}

function filterByEntityType(entityType) {
    state.filters.entityType = entityType;
    applyFilters();
    window.location.hash = '#/';
}

async function renderVectorIndexes() {
    const content = document.getElementById('content');

    // Try to fetch vector indexes
    const data = await fetchAPI('/v1/vectors');
    const indexes = data?.indexes || [];

    const html = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-900">Vector Indexes</h1>
            <p class="mt-1 text-sm text-gray-500">${indexes.length} index${indexes.length !== 1 ? 'es' : ''}</p>
        </div>

        ${indexes.length === 0 ? `
            <div class="text-center py-12 bg-white rounded-lg shadow">
                <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/>
                </svg>
                <h3 class="mt-2 text-sm font-medium text-gray-900">No vector indexes</h3>
                <p class="mt-1 text-sm text-gray-500">Create a vector index to get started with similarity search.</p>
            </div>
        ` : `
            <div class="bg-white shadow rounded-lg overflow-hidden">
                <table class="min-w-full divide-y divide-gray-200">
                    <thead class="bg-gray-50">
                        <tr>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Dimensions</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Metric</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Vectors</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white divide-y divide-gray-200">
                        ${indexes.map(idx => `
                            <tr class="hover:bg-gray-50">
                                <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">${escapeHtml(idx.name)}</td>
                                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${idx.dimensions}</td>
                                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${idx.metric}</td>
                                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${idx.count || 0}</td>
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
            </div>
        `}
    `;

    content.innerHTML = html;
}
