/**
 * Feature Detail Component
 */

function showFeatureDetail(featureName) {
    const feature = state.features.find(f => f.name === featureName);
    if (!feature) {
        showModal(`
            <div class="text-center py-8">
                <p class="text-gray-500">Feature not found: ${escapeHtml(featureName)}</p>
            </div>
        `);
        return;
    }

    state.selectedFeature = feature;
    const dataTypeClass = formatDataType(feature.data_type);
    const statusClass = formatStatus(feature.status);

    const html = `
        <div class="space-y-6">
            <!-- Header -->
            <div class="border-b border-gray-200 pb-4">
                <div class="flex items-start justify-between">
                    <div>
                        <h2 class="text-2xl font-bold text-gray-900">${escapeHtml(feature.name)}</h2>
                        <p class="mt-2 text-gray-600">${escapeHtml(feature.description || 'No description available')}</p>
                    </div>
                    <span class="px-3 py-1 text-sm rounded ${statusClass}">${feature.status || 'active'}</span>
                </div>
            </div>

            <!-- Properties Grid -->
            <div class="grid grid-cols-2 gap-6">
                <div>
                    <h3 class="text-sm font-medium text-gray-500">Data Type</h3>
                    <p class="mt-1">
                        <span class="px-2 py-1 text-sm rounded ${dataTypeClass}">${feature.data_type}</span>
                    </p>
                </div>
                <div>
                    <h3 class="text-sm font-medium text-gray-500">Entity Type</h3>
                    <p class="mt-1 text-sm text-gray-900">${escapeHtml(feature.entity_type || 'Not specified')}</p>
                </div>
                <div>
                    <h3 class="text-sm font-medium text-gray-500">Owner</h3>
                    <p class="mt-1 text-sm text-gray-900">${escapeHtml(feature.owner || 'Not assigned')}</p>
                </div>
                <div>
                    <h3 class="text-sm font-medium text-gray-500">Team</h3>
                    <p class="mt-1 text-sm text-gray-900">${escapeHtml(feature.team || 'Not assigned')}</p>
                </div>
                <div>
                    <h3 class="text-sm font-medium text-gray-500">Category</h3>
                    <p class="mt-1 text-sm text-gray-900">${escapeHtml(feature.category || 'Uncategorized')}</p>
                </div>
                <div>
                    <h3 class="text-sm font-medium text-gray-500">Version</h3>
                    <p class="mt-1 text-sm text-gray-900">v${feature.version || 1}</p>
                </div>
            </div>

            <!-- Tags -->
            ${feature.tags && feature.tags.length > 0 ? `
                <div>
                    <h3 class="text-sm font-medium text-gray-500 mb-2">Tags</h3>
                    <div class="flex flex-wrap gap-2">
                        ${feature.tags.map(tag => `
                            <span class="px-3 py-1 text-sm bg-indigo-50 text-indigo-700 rounded-full">${escapeHtml(tag)}</span>
                        `).join('')}
                    </div>
                </div>
            ` : ''}

            <!-- Metadata -->
            ${feature.metadata && Object.keys(feature.metadata).length > 0 ? `
                <div>
                    <h3 class="text-sm font-medium text-gray-500 mb-2">Metadata</h3>
                    <div class="bg-gray-50 rounded-lg p-4">
                        <dl class="space-y-2">
                            ${Object.entries(feature.metadata).map(([key, value]) => `
                                <div class="flex">
                                    <dt class="w-1/3 text-sm text-gray-500">${escapeHtml(key)}</dt>
                                    <dd class="w-2/3 text-sm text-gray-900">${escapeHtml(String(value))}</dd>
                                </div>
                            `).join('')}
                        </dl>
                    </div>
                </div>
            ` : ''}

            <!-- Source Information -->
            ${feature.source ? `
                <div>
                    <h3 class="text-sm font-medium text-gray-500 mb-2">Source</h3>
                    <div class="bg-gray-50 rounded-lg p-4">
                        <dl class="space-y-2">
                            <div class="flex">
                                <dt class="w-1/3 text-sm text-gray-500">Type</dt>
                                <dd class="w-2/3 text-sm text-gray-900">${escapeHtml(feature.source.type || 'unknown')}</dd>
                            </div>
                            ${feature.source.dbt_model_name ? `
                                <div class="flex">
                                    <dt class="w-1/3 text-sm text-gray-500">dbt Model</dt>
                                    <dd class="w-2/3 text-sm text-gray-900">${escapeHtml(feature.source.dbt_model_name)}</dd>
                                </div>
                            ` : ''}
                            ${feature.source.synced_at ? `
                                <div class="flex">
                                    <dt class="w-1/3 text-sm text-gray-500">Last Synced</dt>
                                    <dd class="w-2/3 text-sm text-gray-900">${new Date(feature.source.synced_at).toLocaleString()}</dd>
                                </div>
                            ` : ''}
                        </dl>
                    </div>
                </div>
            ` : ''}

            <!-- Usage Examples -->
            <div>
                <h3 class="text-sm font-medium text-gray-500 mb-2">Usage Example</h3>
                <div class="bg-gray-900 rounded-lg p-4 overflow-x-auto">
                    <pre class="text-sm text-green-400"><code># Python
from feather_client import FeatherClient

client = FeatherClient("http://localhost:8080")
features = client.get_features(
    entity_id="${escapeHtml(feature.entity_type || 'entity')}:123",
    features=["${escapeHtml(feature.name)}"]
)
print(features["${escapeHtml(feature.name)}"].value)</code></pre>
                </div>
            </div>

            <!-- Actions -->
            <div class="flex space-x-3 pt-4 border-t border-gray-200">
                <button onclick="viewLineage('${escapeHtml(feature.name)}')"
                        class="flex-1 px-4 py-2 text-sm text-indigo-600 border border-indigo-600 rounded hover:bg-indigo-50">
                    View Lineage
                </button>
                <button onclick="closeModal()"
                        class="flex-1 px-4 py-2 text-sm text-gray-700 border border-gray-300 rounded hover:bg-gray-50">
                    Close
                </button>
            </div>
        </div>
    `;

    showModal(html);
}

function viewLineage(featureName) {
    closeModal();
    window.location.hash = `#/lineage?feature=${encodeURIComponent(featureName)}`;
}
