/**
 * Lineage Graph Component
 * Uses Mermaid.js to render feature lineage diagrams
 */

async function renderLineageView() {
    const content = document.getElementById('content');

    // Check for specific feature in URL
    const urlParams = new URLSearchParams(window.location.hash.split('?')[1] || '');
    const selectedFeature = urlParams.get('feature');

    const html = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-900">Feature Lineage</h1>
            <p class="mt-1 text-sm text-gray-500">Visualize feature dependencies and data flow</p>
        </div>

        <!-- Feature Selector -->
        <div class="mb-6 bg-white rounded-lg shadow p-4">
            <label class="block text-sm font-medium text-gray-700 mb-2">Select Feature</label>
            <select id="lineage-feature-select"
                    onchange="updateLineageGraph(this.value)"
                    class="w-full md:w-1/2 px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-indigo-500">
                <option value="">Choose a feature...</option>
                ${state.features.map(f => `
                    <option value="${escapeHtml(f.name)}" ${selectedFeature === f.name ? 'selected' : ''}>
                        ${escapeHtml(f.name)}
                    </option>
                `).join('')}
            </select>
        </div>

        <!-- Lineage Graph -->
        <div id="lineage-container" class="bg-white rounded-lg shadow p-6">
            ${selectedFeature ? '' : `
                <div class="text-center py-12 text-gray-500">
                    <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                    </svg>
                    <p class="mt-2">Select a feature to view its lineage</p>
                </div>
            `}
            <div id="lineage-graph" class="overflow-x-auto"></div>
        </div>

        <!-- Lineage Details -->
        <div id="lineage-details" class="mt-6 hidden">
            <div class="bg-white rounded-lg shadow p-6">
                <h3 class="text-lg font-semibold text-gray-900 mb-4">Dependencies</h3>
                <div id="lineage-dependencies"></div>
            </div>
        </div>
    `;

    content.innerHTML = html;

    // Render graph if feature is selected
    if (selectedFeature) {
        await updateLineageGraph(selectedFeature);
    }
}

async function updateLineageGraph(featureName) {
    const graphContainer = document.getElementById('lineage-graph');
    const detailsContainer = document.getElementById('lineage-details');
    const dependenciesContainer = document.getElementById('lineage-dependencies');

    if (!featureName) {
        graphContainer.innerHTML = `
            <div class="text-center py-12 text-gray-500">
                <p>Select a feature to view its lineage</p>
            </div>
        `;
        detailsContainer.classList.add('hidden');
        return;
    }

    // Update URL
    history.replaceState(null, '', `#/lineage?feature=${encodeURIComponent(featureName)}`);

    // Try to fetch lineage from API
    let lineage = await fetchAPI(`/v1/catalog/features/${encodeURIComponent(featureName)}/lineage`);

    // If no API lineage, generate from feature metadata
    if (!lineage) {
        lineage = generateLocalLineage(featureName);
    }

    // Generate Mermaid diagram
    const mermaidDef = generateMermaidDiagram(featureName, lineage);

    // Render Mermaid graph
    graphContainer.innerHTML = `<div class="mermaid">${mermaidDef}</div>`;
    await mermaid.run({ nodes: graphContainer.querySelectorAll('.mermaid') });

    // Show dependencies details
    if (lineage.upstream && lineage.upstream.length > 0 || lineage.downstream && lineage.downstream.length > 0) {
        detailsContainer.classList.remove('hidden');
        dependenciesContainer.innerHTML = renderDependenciesTable(lineage);
    } else {
        detailsContainer.classList.add('hidden');
    }
}

function generateLocalLineage(featureName) {
    const feature = state.features.find(f => f.name === featureName);
    if (!feature) return { upstream: [], downstream: [] };

    // Generate mock lineage based on naming conventions
    const parts = featureName.split('_');
    const entityType = feature.entity_type || 'unknown';

    // Find potentially related features
    const related = state.features.filter(f =>
        f.name !== featureName &&
        (f.entity_type === entityType || parts.some(p => f.name.includes(p)))
    );

    return {
        upstream: related.slice(0, 2).map(f => ({
            name: f.name,
            type: 'feature',
            entity_type: f.entity_type,
        })),
        downstream: related.slice(2, 4).map(f => ({
            name: f.name,
            type: 'feature',
            entity_type: f.entity_type,
        })),
    };
}

function generateMermaidDiagram(featureName, lineage) {
    const lines = ['graph LR'];
    const nodeId = (name) => name.replace(/[^a-zA-Z0-9]/g, '_');

    // Style definitions
    lines.push('classDef current fill:#6366f1,stroke:#4f46e5,color:#fff');
    lines.push('classDef upstream fill:#f0fdf4,stroke:#22c55e,color:#166534');
    lines.push('classDef downstream fill:#fef3c7,stroke:#f59e0b,color:#92400e');
    lines.push('classDef source fill:#e0e7ff,stroke:#6366f1,color:#3730a3');

    const currentId = nodeId(featureName);
    lines.push(`${currentId}["${featureName}"]`);
    lines.push(`class ${currentId} current`);

    // Add upstream nodes
    if (lineage.upstream) {
        lineage.upstream.forEach((dep, i) => {
            const depId = nodeId(dep.name);
            lines.push(`${depId}["${dep.name}"]`);
            lines.push(`${depId} --> ${currentId}`);

            if (dep.type === 'source') {
                lines.push(`class ${depId} source`);
            } else {
                lines.push(`class ${depId} upstream`);
            }
        });
    }

    // Add downstream nodes
    if (lineage.downstream) {
        lineage.downstream.forEach((dep, i) => {
            const depId = nodeId(dep.name);
            lines.push(`${depId}["${dep.name}"]`);
            lines.push(`${currentId} --> ${depId}`);
            lines.push(`class ${depId} downstream`);
        });
    }

    return lines.join('\n');
}

function renderDependenciesTable(lineage) {
    const upstream = lineage.upstream || [];
    const downstream = lineage.downstream || [];

    return `
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <!-- Upstream -->
            <div>
                <h4 class="text-sm font-medium text-gray-700 mb-2">Upstream (${upstream.length})</h4>
                ${upstream.length === 0 ? '<p class="text-sm text-gray-500">No upstream dependencies</p>' : `
                    <ul class="space-y-2">
                        ${upstream.map(dep => `
                            <li class="flex items-center justify-between p-2 bg-green-50 rounded">
                                <span class="text-sm font-medium text-green-800">${escapeHtml(dep.name)}</span>
                                <span class="text-xs text-green-600">${escapeHtml(dep.type || 'feature')}</span>
                            </li>
                        `).join('')}
                    </ul>
                `}
            </div>

            <!-- Downstream -->
            <div>
                <h4 class="text-sm font-medium text-gray-700 mb-2">Downstream (${downstream.length})</h4>
                ${downstream.length === 0 ? '<p class="text-sm text-gray-500">No downstream dependencies</p>' : `
                    <ul class="space-y-2">
                        ${downstream.map(dep => `
                            <li class="flex items-center justify-between p-2 bg-yellow-50 rounded">
                                <span class="text-sm font-medium text-yellow-800">${escapeHtml(dep.name)}</span>
                                <span class="text-xs text-yellow-600">${escapeHtml(dep.type || 'feature')}</span>
                            </li>
                        `).join('')}
                    </ul>
                `}
            </div>
        </div>
    `;
}
